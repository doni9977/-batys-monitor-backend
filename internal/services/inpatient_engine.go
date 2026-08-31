package services

import (
	"fmt"
	"log"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
)

// RunInpatientEngine — оркестратор движка стационара.
func RunInpatientEngine(jobID uint) {
	if database.DB == nil {
		log.Println("DB is nil, пропускаем inpatient engine")
		return
	}

	if err := markJobRunning(jobID); err != nil {
		log.Printf("Не удалось перевести job=%d в running: %v", jobID, err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("panic: %v", recovered)
			log.Printf("Паника в inpatient engine job=%d: %v", jobID, recovered)
			if updateErr := markJobFailed(jobID, err); updateErr != nil {
				log.Printf("Не удалось перевести job=%d в failed после panic: %v", jobID, updateErr)
			}
		}
	}()

	// Чистим риски прошлых запусков стационара (только свои индикаторы S1–S5).
	if err := database.DB.Exec(
		"DELETE FROM detected_risks WHERE indicator IN ('S1','S2','S3','S4','S5')",
	).Error; err != nil {
		log.Printf("Не удалось очистить старые риски стационара: %v", err)
	}

	risks := make([]models.DetectedRisk, 0, 1000)

	risks = append(risks, collectS3(jobID)...)
	risks = append(risks, collectS2(jobID)...)
	risks = append(risks, collectS4(jobID)...)
	risks = append(risks, collectS1(jobID)...)
	risks = append(risks, collectS5(jobID)...)

	if len(risks) == 0 {
		log.Println("Стационар: риски не найдены")
		if err := markJobDone(jobID, 0); err != nil {
			log.Printf("Не удалось перевести job=%d в done: %v", jobID, err)
		}
		return
	}

	if err := database.DB.CreateInBatches(risks, 500).Error; err != nil {
		log.Printf("Ошибка сохранения рисков стационара: %v", err)
		if updateErr := markJobFailed(jobID, err); updateErr != nil {
			log.Printf("Не удалось перевести job=%d в failed: %v", jobID, updateErr)
		}
		return
	}

	if err := markJobDone(jobID, len(risks)); err != nil {
		log.Printf("Не удалось перевести job=%d в done: %v", jobID, err)
	}

	log.Printf("Стационар: сохранено %d рисков в detected_risks", len(risks))
}

// collectS3 — Фиктивный круглосуточный стационар (bed_days<=1, не умер/переведён).
func collectS3(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		PatientName   string
		PatientDOB    string
		HospitalName  string
		Department    string
		DoctorName    string
		AdmissionDate time.Time
		DischargeDate time.Time
		BedDays       int
		Outcome       string
		ICD10Code     string
		Diagnosis     string
		CareType      string
		Amount        float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT patient_name, patient_dob, hospital_name, department, doctor_name,
               admission_date, discharge_date, bed_days, outcome, icd10_code,
               diagnosis, care_type, amount
        FROM inpatient_records
        WHERE care_type ILIKE '%круглосуточный%'
          AND bed_days <= 1
          AND COALESCE(outcome, '') NOT IN ('Умер', 'Переведен')
    `).Scan(&rows).Error
	if err != nil {
		log.Printf("S3 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"patient_name":   r.PatientName,
			"patient_dob":    r.PatientDOB,
			"hospital_name":  r.HospitalName,
			"department":     r.Department,
			"doctor_name":    r.DoctorName,
			"admission_date": r.AdmissionDate.Format("2006-01-02"),
			"discharge_date": r.DischargeDate.Format("2006-01-02"),
			"bed_days":       r.BedDays,
			"outcome":        r.Outcome,
			"icd10_code":     r.ICD10Code,
			"diagnosis":      r.Diagnosis,
			"care_type":      r.CareType,
			"amount":         r.Amount,
			"reason":         "Круглосуточный стационар с пребыванием ≤1 койко-дня",
		}
		results = append(results, makeDetectedRisk(
			jobID, "S3", r.HospitalName, r.DoctorName, r.PatientName, r.AdmissionDate, r.Amount, details,
		))
	}
	log.Printf("S3: найдено %d рисков", len(results))
	return results
}

// collectS2 — Дробление госпитализаций (повтор с тем же диагнозом в течение 0–3 дней).
func collectS2(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		PatientName   string
		PatientDOB    string
		ICD10Code     string
		HospitalName  string
		DoctorName    string
		AdmissionDate time.Time
		PrevDischarge time.Time
		GapDays       int
		Amount        float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        WITH ordered AS (
            SELECT patient_name, patient_dob, icd10_code, hospital_name, doctor_name,
                   admission_date, amount,
                   LAG(discharge_date) OVER (
                       PARTITION BY patient_name, patient_dob, icd10_code
                       ORDER BY admission_date
                   ) AS prev_discharge
            FROM inpatient_records
            WHERE COALESCE(patient_name, '') <> '' AND COALESCE(icd10_code, '') <> ''
        )
        SELECT patient_name, patient_dob, icd10_code, hospital_name, doctor_name,
               admission_date, prev_discharge,
               (admission_date::date - prev_discharge::date) AS gap_days, amount
        FROM ordered
        WHERE prev_discharge IS NOT NULL
          AND (admission_date::date - prev_discharge::date) BETWEEN 0 AND 3
    `).Scan(&rows).Error
	if err != nil {
		log.Printf("S2 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"patient_name":       r.PatientName,
			"patient_dob":        r.PatientDOB,
			"icd10_code":         r.ICD10Code,
			"hospital_name":      r.HospitalName,
			"doctor_name":        r.DoctorName,
			"prev_discharge":     r.PrevDischarge.Format("2006-01-02"),
			"new_admission_date": r.AdmissionDate.Format("2006-01-02"),
			"gap_days":           r.GapDays,
			"amount":             r.Amount,
			"reason":             "Повторная госпитализация с тем же диагнозом в течение 0–3 дней",
		}
		results = append(results, makeDetectedRisk(
			jobID, "S2", r.HospitalName, r.DoctorName, r.PatientName, r.AdmissionDate, r.Amount, details,
		))
	}
	log.Printf("S2: найдено %d рисков", len(results))
	return results
}

// collectS4 — Аномалия экстренных (у врача >90% экстренных при >10 пациентах).
func collectS4(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		DoctorName     string
		Department     string
		HospitalName   string
		Total          int
		EmergencyCount int
		EmergencyPct   float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT doctor_name, department, MAX(hospital_name) AS hospital_name,
               COUNT(*) AS total, SUM(is_emergency) AS emergency_count,
               ROUND(SUM(is_emergency)::numeric * 100.0 / COUNT(*), 1) AS emergency_pct
        FROM inpatient_records
        WHERE COALESCE(doctor_name, '') <> ''
          AND department NOT ILIKE '%реаним%'
        GROUP BY doctor_name, department
        HAVING COUNT(*) > 10
           AND SUM(is_emergency)::numeric * 100.0 / COUNT(*) > 90
    `).Scan(&rows).Error
	if err != nil {
		log.Printf("S4 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"doctor_name":        r.DoctorName,
			"department":         r.Department,
			"hospital_name":      r.HospitalName,
			"total_patients":     r.Total,
			"emergency_patients": r.EmergencyCount,
			"emergency_percent":  r.EmergencyPct,
			"reason":             "Аномально высокая доля экстренных госпитализаций у врача (>90%)",
		}
		results = append(results, makeDetectedRisk(
			jobID, "S4", r.HospitalName, r.DoctorName, "", time.Now(), 0, details,
		))
	}
	log.Printf("S4: найдено %d рисков", len(results))
	return results
}

// collectS1 — Кросс-чек стационара и поликлиники.
//
// Суть: пациент лежал в круглосуточном стационаре (admission_date … discharge_date),
// а в поликлинике (service_records) в эти же дни у него оформлена услуга —
// физически невозможно, значит приписка.
//
// Механика (JOIN двух таблиц):
//   - связываем по нормализованному имени (regexp_replace убирает точки/пробелы,
//     upper — регистр) + дате рождения;
//   - риск, если дата услуги поликлиники строго внутри периода госпитализации.
//
// Сумма риска = сумма амбулаторной услуги (в поликлинике).
func collectS1(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		PatientName   string
		PatientDOB    string
		HospitalName  string
		DoctorName    string
		AdmissionDate time.Time
		DischargeDate time.Time
		PoliClinic    string
		ServiceName   string
		ServiceDate   time.Time
		ServiceAmount float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT DISTINCT
            i.patient_name,
            i.patient_dob,
            i.hospital_name,
            i.doctor_name,
            i.admission_date,
            i.discharge_date,
            s.clinic_name  AS poli_clinic,
            s.service_name,
            s.service_date,
            s.amount        AS service_amount
        FROM inpatient_records i
        JOIN service_records s
          ON replace(replace(upper(i.patient_name), '.', ''), ' ', '')
           = replace(replace(upper(s.patient_name), '.', ''), ' ', '')
         AND i.patient_dob = s.patient_dob
        WHERE s.service_date::date > i.admission_date::date
          AND s.service_date::date < i.discharge_date::date
    `).Scan(&rows).Error
	if err != nil {
		log.Printf("S1 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"patient_name":   r.PatientName,
			"patient_dob":    r.PatientDOB,
			"hospital_name":  r.HospitalName, // где лежал (стационар)
			"poli_clinic":    r.PoliClinic,   // где "оказали" услугу (поликлиника)
			"service_name":   r.ServiceName,
			"service_date":   r.ServiceDate.Format("2006-01-02"),
			"admission_date": r.AdmissionDate.Format("2006-01-02"),
			"discharge_date": r.DischargeDate.Format("2006-01-02"),
			"amount":         r.ServiceAmount,
			"reason":         "Услуга в поликлинике в дни, когда пациент лежал в стационаре",
		}
		results = append(results, makeDetectedRisk(
			jobID, "S1", r.HospitalName, r.DoctorName, r.PatientName, r.ServiceDate, r.ServiceAmount, details,
		))
	}
	log.Printf("S1: найдено %d рисков", len(results))
	return results
}

// collectS5 — "Услуги после смерти".
//
// Суть: пациент умер в стационаре (outcome='Умер', дата смерти = дата выписки),
// а в поликлинике ему оформлены услуги ПОСЛЕ даты смерти — счёт за мёртвого.
//
// Механика (JOIN стационар × поликлиника):
//   - берём умерших пациентов стационара;
//   - связываем с поликлиникой по нормализованному имени + дате рождения;
//   - риск, если дата услуги поликлиники строго больше даты смерти.
//
// Сумма риска = сумма услуги, выставленной после смерти.
func collectS5(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	type row struct {
		PatientName   string
		PatientDOB    string
		HospitalName  string
		DoctorName    string
		DeathDate     time.Time
		PoliClinic    string
		ServiceName   string
		ServiceDate   time.Time
		ServiceAmount float64
	}

	rows := []row{}
	err := database.DB.Raw(`
        SELECT
            i.patient_name,
            i.patient_dob,
            i.hospital_name,
            i.doctor_name,
            i.discharge_date AS death_date,
            s.clinic_name    AS poli_clinic,
            s.service_name,
            s.service_date,
            s.amount          AS service_amount
        FROM inpatient_records i
        JOIN service_records s
          ON replace(replace(upper(i.patient_name), '.', ''), ' ', '')
           = replace(replace(upper(s.patient_name), '.', ''), ' ', '')
         AND i.patient_dob = s.patient_dob
        WHERE i.outcome = 'Умер'
          AND s.service_date::date > i.discharge_date::date
    `).Scan(&rows).Error
	if err != nil {
		log.Printf("S5 query failed: %v", err)
		return results
	}

	for _, r := range rows {
		details := map[string]interface{}{
			"patient_name":  r.PatientName,
			"patient_dob":   r.PatientDOB,
			"hospital_name": r.HospitalName,
			"death_date":    r.DeathDate.Format("2006-01-02"),
			"poli_clinic":   r.PoliClinic,
			"service_name":  r.ServiceName,
			"service_date":  r.ServiceDate.Format("2006-01-02"),
			"amount":        r.ServiceAmount,
			"reason":        "Услуга в поликлинике после зафиксированной даты смерти пациента",
		}
		results = append(results, makeDetectedRisk(
			jobID, "S5", r.HospitalName, r.DoctorName, r.PatientName, r.ServiceDate, r.ServiceAmount, details,
		))
	}
	log.Printf("S5: найдено %d рисков", len(results))
	return results
}