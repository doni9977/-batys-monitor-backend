package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"gorm.io/datatypes"
)

func EnqueueRiskCalculationJob(sourceFile string, loadedRecords int64) (*models.RiskJob, error) {
	job := &models.RiskJob{
		Status:        models.RiskJobStatusQueued,
		SourceFile:    sourceFile,
		LoadedRecords: loadedRecords,
	}

	if err := database.DB.Create(job).Error; err != nil {
		return nil, err
	}

	return job, nil
}

func RunAllRiskEngines(jobID uint) {
	if database.DB == nil {
		log.Println("DB is nil, skipping risk engine")
		return
	}

	if err := markJobRunning(jobID); err != nil {
		log.Printf("Не удалось перевести job=%d в running: %v", jobID, err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("panic: %v", recovered)
			log.Printf("Паника в risk engine job=%d: %v", jobID, recovered)
			if updateErr := markJobFailed(jobID, err); updateErr != nil {
				log.Printf("Не удалось перевести job=%d в failed после panic: %v", jobID, updateErr)
			}
		}
	}()

	risks := make([]models.DetectedRisk, 0, 5000)

	risks = append(risks, collectA1(jobID)...)
	risks = append(risks, collectA2(jobID)...)
	risks = append(risks, collectA3(jobID)...)
	risks = append(risks, collectA4(jobID)...)
	risks = append(risks, collectA7(jobID)...)
	risks = append(risks, collectA8(jobID)...)
	risks = append(risks, collectA10(jobID)...)

	if len(risks) == 0 {
		log.Println("Нарушения не найдены")
		if err := markJobDone(jobID, 0); err != nil {
			log.Printf("Не удалось перевести job=%d в done: %v", jobID, err)
		}
		return
	}

	if err := database.DB.CreateInBatches(risks, 500).Error; err != nil {
		log.Printf("Ошибка сохранения обнаруженных рисков: %v", err)
		if updateErr := markJobFailed(jobID, err); updateErr != nil {
			log.Printf("Не удалось перевести job=%d в failed: %v", jobID, updateErr)
		}
		return
	}

	if err := markJobDone(jobID, len(risks)); err != nil {
		log.Printf("Не удалось перевести job=%d в done: %v", jobID, err)
	}

	log.Printf("Успешно сохранено %d рисков в detected_risks", len(risks))
}

func markJobRunning(jobID uint) error {
	if jobID == 0 {
		return nil
	}

	now := time.Now()
	return database.DB.Model(&models.RiskJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":        models.RiskJobStatusRunning,
			"started_at":    now,
			"finished_at":   nil,
			"error_message": "",
		}).Error
}

func markJobDone(jobID uint, risksFound int) error {
	if jobID == 0 {
		return nil
	}

	now := time.Now()
	return database.DB.Model(&models.RiskJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":        models.RiskJobStatusDone,
			"finished_at":   now,
			"risks_found":   risksFound,
			"error_message": "",
		}).Error
}

func markJobFailed(jobID uint, cause error) error {
	if jobID == 0 {
		return nil
	}

	now := time.Now()
	errMessage := ""
	if cause != nil {
		errMessage = cause.Error()
	}

	return database.DB.Model(&models.RiskJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":        models.RiskJobStatusFailed,
			"finished_at":   now,
			"error_message": errMessage,
		}).Error
}

func makeDetectedRisk(
	jobID uint,
	indicator string,
	clinicName string,
	doctorName string,
	patientIIN string,
	riskDate time.Time,
	amount float64,
	details map[string]interface{},
) models.DetectedRisk {
	body, _ := json.Marshal(details)

	return models.DetectedRisk{
		JobID:      jobID,
		Indicator:  indicator,
		ClinicName: clinicName,
		DoctorName: doctorName,
		PatientIIN: patientIIN,
		RiskDate:   riskDate,
		Amount:     amount,
		Details:    datatypes.JSON(body),
	}
}

func collectA1(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		ClinicName    string
		DoctorName    string
		PatientIIN    string
		PatientGender string
		ServiceDate   time.Time
		ServiceCode   string
		ServiceName   string
		PatientAge    int
		Reason        string
		MinAge        int
		MaxAge        int
	}

	rows := []row{}
	err := database.DB.Raw(`
        WITH parsed AS (
            SELECT
                sr.clinic_name,
                sr.doctor_name,
                sr.patient_iin,
                sr.patient_gender,
                sr.service_date,
                sr.service_code,
                sr.service_name,
                CASE
                    WHEN BTRIM(sr.patient_dob) ~ '^\d{2}\.\d{2}\.\d{4}$'
                        THEN TO_DATE(BTRIM(sr.patient_dob), 'DD.MM.YYYY')
                    WHEN BTRIM(sr.patient_dob) ~ '^\d{4}-\d{2}-\d{2}$'
                        THEN TO_DATE(BTRIM(sr.patient_dob), 'YYYY-MM-DD')
                    ELSE NULL
                END AS dob,
                sc.min_age,
                sc.max_age
            FROM service_records AS sr
            JOIN service_classifiers AS sc ON sc.code = sr.service_code
        )
        SELECT
            clinic_name,
            doctor_name,
            patient_iin,
            patient_gender,
            service_date::date AS service_date,
            service_code,
            service_name,
            EXTRACT(YEAR FROM AGE(service_date, dob))::int AS patient_age,
            'Нарушение возраста' AS reason,
            min_age,
            max_age
        FROM parsed
        WHERE dob IS NOT NULL
          AND service_date IS NOT NULL
          AND (
            (min_age > 0 AND EXTRACT(YEAR FROM AGE(service_date, dob))::int < min_age)
            OR
            (max_age > 0 AND EXTRACT(YEAR FROM AGE(service_date, dob))::int > max_age)
          )
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A1 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"clinic_name":    r.ClinicName,
			"doctor_name":    r.DoctorName,
			"patient_iin":    r.PatientIIN,
			"patient_gender": r.PatientGender,
			"patient_age":    r.PatientAge,
			"service_code":   r.ServiceCode,
			"service_name":   r.ServiceName,
			"service_date":   r.ServiceDate.Format("2006-01-02"),
			"reason":         r.Reason,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A1",
			r.ClinicName,
			r.DoctorName,
			r.PatientIIN,
			r.ServiceDate,
			0,
			details,
		))
	}

	return results
}

func collectA2(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		ClinicName    string
		DoctorName    string
		PatientIIN    string
		PatientGender string
		ServiceCode   string
		ServiceName   string
		ServiceDate   time.Time
		Reason        string
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT
            sr.clinic_name,
            sr.doctor_name,
            sr.patient_iin,
            sr.patient_gender,
            sr.service_code,
            sr.service_name,
            sr.service_date::date AS service_date,
            'Нарушение пола' AS reason
        FROM service_records AS sr
        JOIN service_classifiers AS sc ON sc.code = sr.service_code
        WHERE sr.service_date IS NOT NULL
          AND BTRIM(sr.patient_gender) <> ''
          AND (
            (LOWER(BTRIM(sc.gender_restriction)) = 'женщина' AND LOWER(BTRIM(sr.patient_gender)) <> 'женщина')
            OR
            (LOWER(BTRIM(sc.gender_restriction)) = 'мужчина' AND LOWER(BTRIM(sr.patient_gender)) <> 'мужчина')
          )
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A2 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"clinic_name":    r.ClinicName,
			"doctor_name":    r.DoctorName,
			"patient_iin":    r.PatientIIN,
			"patient_gender": r.PatientGender,
			"service_code":   r.ServiceCode,
			"service_name":   r.ServiceName,
			"service_date":   r.ServiceDate.Format("2006-01-02"),
			"reason":         r.Reason,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A2",
			r.ClinicName,
			r.DoctorName,
			r.PatientIIN,
			r.ServiceDate,
			0,
			details,
		))
	}

	return results
}

func collectA3(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		DoctorName   string
		ServiceDate  time.Time
		ServiceCount int
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT
            doctor_name,
            service_date::date AS service_date,
            COUNT(*) AS service_count
        FROM service_records
        GROUP BY doctor_name, service_date::date
        HAVING COUNT(*) > 200
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A3 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"doctor_name":   r.DoctorName,
			"service_date":  r.ServiceDate.Format("2006-01-02"),
			"service_count": r.ServiceCount,
			"threshold":     200,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A3",
			"",
			r.DoctorName,
			"",
			r.ServiceDate,
			float64(r.ServiceCount-200),
			details,
		))
	}

	return results
}

func collectA4(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		PatientIIN       string
		ServiceCode      string
		ServiceName      string
		RiskDate         time.Time
		TotalCount       int
		TotalAmount      float64
		ClinicCount      int
		DifferentClinics bool
		Clinics          string
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT
            patient_iin,
            service_code,
            MIN(service_name) AS service_name,
            service_date::date AS risk_date,
            COUNT(*) AS total_count,
            COALESCE(SUM(amount), 0) AS total_amount,
            COUNT(DISTINCT clinic_name) AS clinic_count,
            COUNT(DISTINCT clinic_name) > 1 AS different_clinics,
            STRING_AGG(DISTINCT clinic_name, ', ' ORDER BY clinic_name) AS clinics
        FROM service_records
        GROUP BY patient_iin, service_code, service_date::date
        HAVING COUNT(*) > 1
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A4 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"patient_iin":       r.PatientIIN,
			"service_code":      r.ServiceCode,
			"service_name":      r.ServiceName,
			"date":              r.RiskDate.Format("2006-01-02"),
			"total_count":       r.TotalCount,
			"total_amount":      r.TotalAmount,
			"clinic_count":      r.ClinicCount,
			"different_clinics": r.DifferentClinics,
			"clinics":           r.Clinics,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A4",
			"",
			"",
			r.PatientIIN,
			r.RiskDate,
			r.TotalAmount,
			details,
		))
	}

	return results
}

func collectA7(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		PatientIIN     string
		ServiceCode    string
		ServiceName    string
		Year           int
		TotalQuantity  int
		AllowedPerYear int
		TotalAmount    float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT
            sr.patient_iin,
            sr.service_code,
            MIN(sr.service_name) AS service_name,
            EXTRACT(YEAR FROM sr.service_date)::int AS year,
            SUM(GREATEST(sr.quantity, 1))::int AS total_quantity,
            MAX(sc.max_per_year) AS allowed_per_year,
            COALESCE(SUM(sr.amount), 0) AS total_amount
        FROM service_records AS sr
        JOIN service_classifiers AS sc ON sc.code = sr.service_code
        WHERE sr.service_date IS NOT NULL
          AND sc.max_per_year > 0
        GROUP BY sr.patient_iin, sr.service_code, EXTRACT(YEAR FROM sr.service_date)
        HAVING SUM(GREATEST(sr.quantity, 1)) > MAX(sc.max_per_year)
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A7 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		riskDate := time.Date(r.Year, 1, 1, 0, 0, 0, 0, time.UTC)

		details := map[string]interface{}{
			"patient_iin":      r.PatientIIN,
			"service_code":     r.ServiceCode,
			"service_name":     r.ServiceName,
			"year":             r.Year,
			"total_quantity":   r.TotalQuantity,
			"allowed_per_year": r.AllowedPerYear,
			"total_amount":     r.TotalAmount,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A7",
			"",
			"",
			r.PatientIIN,
			riskDate,
			r.TotalAmount,
			details,
		))
	}

	return results
}

func collectA8(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		ClinicName    string
		DoctorName    string
		PatientIIN    string
		ServiceCode   string
		ServiceName   string
		ServiceDate   time.Time
		Quantity      int
		ActualAmount  float64
		AllowedAmount float64
		ExcessAmount  float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT
            sr.clinic_name,
            sr.doctor_name,
            sr.patient_iin,
            sr.service_code,
            sr.service_name,
            sr.service_date::date AS service_date,
            GREATEST(sr.quantity, 1) AS quantity,
            sr.amount AS actual_amount,
            (sc.tariff * GREATEST(sr.quantity, 1)) AS allowed_amount,
            (sr.amount - sc.tariff * GREATEST(sr.quantity, 1)) AS excess_amount
        FROM service_records AS sr
        JOIN service_classifiers AS sc ON sc.code = sr.service_code
        WHERE sc.tariff > 0
          AND sr.amount > sc.tariff * GREATEST(sr.quantity, 1)
        ORDER BY excess_amount DESC
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A8 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"clinic_name":    r.ClinicName,
			"doctor_name":    r.DoctorName,
			"patient_iin":    r.PatientIIN,
			"service_code":   r.ServiceCode,
			"service_name":   r.ServiceName,
			"service_date":   r.ServiceDate.Format("2006-01-02"),
			"quantity":       r.Quantity,
			"actual_amount":  r.ActualAmount,
			"allowed_amount": r.AllowedAmount,
			"excess_amount":  r.ExcessAmount,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A8",
			r.ClinicName,
			r.DoctorName,
			r.PatientIIN,
			r.ServiceDate,
			r.ExcessAmount,
			details,
		))
	}

	return results
}

func collectA10(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		DoctorName              string
		PreviousServiceCode     string
		PreviousServiceName     string
		PreviousServiceDate     time.Time
		ServiceCode             string
		ServiceName             string
		ServiceDate             time.Time
		ActualIntervalMinutes   int
		RequiredIntervalMinutes int
	}

	rows := []row{}
	err := database.DB.Raw(`
        WITH ordered_services AS (
            SELECT
                sr.doctor_name,
                sr.service_code,
                sr.service_name,
                sr.service_date,
                LAG(sr.service_code) OVER physician_services AS previous_service_code,
                LAG(sr.service_name) OVER physician_services AS previous_service_name,
                LAG(sr.service_date) OVER physician_services AS previous_service_date,
                LAG(sc.norm_minutes) OVER physician_services AS required_interval_minutes
            FROM service_records AS sr
            JOIN service_classifiers AS sc ON sc.code = sr.service_code
            WHERE sr.doctor_name <> ''
              AND sr.service_date::time <> TIME '00:00:00'
              AND sc.norm_minutes > 0
            WINDOW physician_services AS (PARTITION BY sr.doctor_name ORDER BY sr.service_date, sr.id)
        )
        SELECT
            doctor_name,
            previous_service_code,
            previous_service_name,
            previous_service_date::timestamp AS previous_service_date,
            service_code,
            service_name,
            service_date::timestamp AS service_date,
            EXTRACT(EPOCH FROM (service_date - previous_service_date))::int / 60 AS actual_interval_minutes,
            required_interval_minutes
        FROM ordered_services
        WHERE previous_service_date IS NOT NULL
          AND service_date > previous_service_date
          AND service_date < previous_service_date + required_interval_minutes * INTERVAL '1 minute'
        ORDER BY (required_interval_minutes - EXTRACT(EPOCH FROM (service_date - previous_service_date))::int / 60) DESC
    `).Scan(&rows).Error

	if err != nil {
		log.Printf("A10 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		amount := float64(r.RequiredIntervalMinutes - r.ActualIntervalMinutes)

		details := map[string]interface{}{
			"doctor_name":               r.DoctorName,
			"previous_service_code":     r.PreviousServiceCode,
			"previous_service_name":     r.PreviousServiceName,
			"previous_service_date":     r.PreviousServiceDate.Format("2006-01-02 15:04:05"),
			"service_code":              r.ServiceCode,
			"service_name":              r.ServiceName,
			"service_date":              r.ServiceDate.Format("2006-01-02 15:04:05"),
			"actual_interval_minutes":   r.ActualIntervalMinutes,
			"required_interval_minutes": r.RequiredIntervalMinutes,
		}

		results = append(results, makeDetectedRisk(
			jobID,
			"A10",
			"",
			r.DoctorName,
			"",
			r.ServiceDate,
			amount,
			details,
		))
	}

	return results
}
