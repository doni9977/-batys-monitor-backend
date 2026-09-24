package services

import (
	"log"
	"time"

	"encoding/json"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"gorm.io/datatypes"
)

// RunAllNrRiskEngines запускает все NR-алгоритмы для указанного job.
func RunAllNrRiskEngines(jobID uint) {
	log.Printf("[NR Engine] Запуск алгоритмов NR1-NR5 для job_id=%d", jobID)
	if err := markJobRunning(jobID); err != nil {
		if err == ErrJobCancelled {
			return
		}
		log.Printf("[NR Engine] Не удалось перевести job=%d в running: %v", jobID, err)
	}
	if isJobCancelled(jobID) {
		return
	}

	// Очищаем старые NR-риски для этого job
	database.DB.Where("job_id = ? AND indicator LIKE 'NR%'", jobID).
		Delete(&models.DetectedRisk{})

	var allRisks []models.DetectedRisk
	collectors := []func(uint) []models.DetectedRisk{collectNR1, collectNR2, collectNR3, collectNR4, collectNR5}
	for _, collect := range collectors {
		if isJobCancelled(jobID) {
			return
		}
		allRisks = append(allRisks, collect(jobID)...)
	}

	if !isJobCancelled(jobID) && len(allRisks) > 0 {
		res := database.DB.CreateInBatches(&allRisks, 500)
		if res.Error != nil {
			log.Printf("[NR Engine] Ошибка сохранения рисков: %v", res.Error)
		}
	}
	if isJobCancelled(jobID) {
		deleteJobRisks(jobID)
		return
	}

	// Обновляем счётчик в job
	database.DB.Model(&models.RiskJob{}).Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"risks_found": len(allRisks),
			"status":      models.RiskJobStatusDone,
			"finished_at": time.Now(),
		})

	log.Printf("[NR Engine] Завершено. Найдено рисков: %d", len(allRisks))
}

func nrJSON(m map[string]interface{}) datatypes.JSON {
	b, _ := json.Marshal(m)
	return datatypes.JSON(b)
}

func makeNrRisk(jobID uint, indicator, clinicName, doctorName, patientIIN string,
	riskDate time.Time, amount float64, details map[string]interface{}) models.DetectedRisk {
	return models.DetectedRisk{
		JobID:      jobID,
		Indicator:  indicator,
		ClinicName: clinicName, // переиспользуем как "Название компании"
		DoctorName: doctorName, // переиспользуем как "ФИО руководителя"
		PatientIIN: patientIIN, // переиспользуем как "БИН компании"
		RiskDate:   riskDate,
		Amount:     amount,
		Details:    nrJSON(details),
	}
}

// ─── NR1: Фиктивное присутствие ─────────────────────────────────────────────
// Нерезидент не въезжал в РК к дате регистрации компании,
// либо въехал ПОСЛЕ даты регистрации (дистанционная регистрация по доверенности).
func collectNR1(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	// Получаем все NR-записи для этого job
	var nrRecords []models.NrRecord
	database.DB.Where("job_id = ?", jobID).Find(&nrRecords)

	for _, nr := range nrRecords {
		if nr.DirectorIIN == "" || nr.RegDate.IsZero() {
			continue
		}

		// Ищем записи Беркут для этого нерезидента
		var berkutRecords []models.BerkutRecord
		database.DB.Where("director_iin = ?", nr.DirectorIIN).Find(&berkutRecords)

		riskDetected := false
		riskReason := ""

		if len(berkutRecords) == 0 {
			// Никогда не въезжал в РК — явное фиктивное присутствие
			riskDetected = true
			riskReason = "Нерезидент не имеет записей о въезде в РК (Беркут)"
		} else {
			// Проверяем: был ли в РК на дату регистрации?
			wasInRK := false
			for _, b := range berkutRecords {
				if !b.EntryDate.IsZero() && !b.ExitDate.IsZero() {
					// Был в РК в период от EntryDate до ExitDate
					if !nr.RegDate.Before(b.EntryDate) && !nr.RegDate.After(b.ExitDate) {
						wasInRK = true
						break
					}
				}
			}
			if !wasInRK {
				riskDetected = true
				riskReason = "Дата регистрации компании не совпадает с периодом пребывания нерезидента в РК"
			}
		}

		if riskDetected {
			details := map[string]interface{}{
				"company_name":     nr.FullName,
				"bin":              nr.BIN,
				"director_name":    nr.DirectorName,
				"director_iin":     nr.DirectorIIN,
				"director_country": nr.DirectorCountry,
				"reg_date":         nr.RegDate.Format("2006-01-02"),
				"legal_address":    nr.LegalAddress,
				"reason":           riskReason,
				"notary":           nr.Notary,
				"translator":       nr.Translator,
			}
			results = append(results, makeNrRisk(
				jobID, "NR1",
				nr.FullName, nr.DirectorName, nr.BIN,
				nr.RegDate, 0, details,
			))
		}
	}

	log.Printf("[NR1] Найдено рисков: %d", len(results))
	return results
}

// ─── NR2: Транзитный туризм ──────────────────────────────────────────────────
// Пребывание ≤ 3 дней И тот же ГРНЗ использовало 3+ разных нерезидентов.
func collectNR2(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	// Группируем Беркут по ГРНЗ
	type grpRow struct {
		VehiclePlate string
		Count        int64
	}
	var grpRows []grpRow
	database.DB.Model(&models.BerkutRecord{}).
		Select("vehicle_plate, COUNT(DISTINCT director_iin) as count").
		Where("vehicle_plate != ''").
		Group("vehicle_plate").
		Having("COUNT(DISTINCT director_iin) >= 3").
		Scan(&grpRows)

	// Для каждого подозрительного ГРНЗ — найти нерезидентов с коротким визитом
	for _, grp := range grpRows {
		var berkuts []models.BerkutRecord
		database.DB.Where("vehicle_plate = ?", grp.VehiclePlate).Find(&berkuts)

		for _, b := range berkuts {
			if b.EntryDate.IsZero() || b.ExitDate.IsZero() {
				continue
			}
			stayDays := int(b.ExitDate.Sub(b.EntryDate).Hours() / 24)
			if stayDays > 3 {
				continue
			}

			// Найти компанию этого нерезидента
			var nr models.NrRecord
			if err := database.DB.Where("job_id = ? AND director_iin = ?", jobID, b.DirectorIIN).
				First(&nr).Error; err != nil {
				continue
			}

			details := map[string]interface{}{
				"company_name":       nr.FullName,
				"bin":                nr.BIN,
				"director_name":      b.DirectorName,
				"director_iin":       b.DirectorIIN,
				"vehicle_plate":      b.VehiclePlate,
				"entry_date":         b.EntryDate.Format("2006-01-02"),
				"exit_date":          b.ExitDate.Format("2006-01-02"),
				"stay_days":          stayDays,
				"shared_plate_count": grp.Count,
				"crossing_point":     b.CrossingPoint,
				"reason":             "Короткий визит (≤3 дней) + ГРНЗ использовало 3+ нерезидентов",
			}
			results = append(results, makeNrRisk(
				jobID, "NR2",
				nr.FullName, b.DirectorName, nr.BIN,
				nr.RegDate, 0, details,
			))
		}
	}

	log.Printf("[NR2] Найдено рисков: %d", len(results))
	return results
}

// ─── NR3: Аффилированные сети посредников ────────────────────────────────────
// Пара (нотариус + переводчик) участвовала в регистрации > 3 компаний.
func collectNR3(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	// Группируем по паре нотариус+переводчик
	type pairRow struct {
		Notary     string
		Translator string
		Count      int64
	}
	var pairs []pairRow
	database.DB.Model(&models.NrRecord{}).
		Select("notary, translator, COUNT(id) as count").
		Where("job_id = ? AND notary != '' AND translator != ''", jobID).
		Group("notary, translator").
		Having("COUNT(id) > 3").
		Scan(&pairs)

	// Для каждой подозрительной пары — получаем все компании
	for _, pair := range pairs {
		var nrRecords []models.NrRecord
		database.DB.Where("job_id = ? AND notary = ? AND translator = ?",
			jobID, pair.Notary, pair.Translator).Find(&nrRecords)

		for _, nr := range nrRecords {
			details := map[string]interface{}{
				"company_name":  nr.FullName,
				"bin":           nr.BIN,
				"director_name": nr.DirectorName,
				"director_iin":  nr.DirectorIIN,
				"notary":        nr.Notary,
				"translator":    nr.Translator,
				"pair_count":    pair.Count,
				"reg_date":      nr.RegDate.Format("2006-01-02"),
				"legal_address": nr.LegalAddress,
				"reason":        "Устойчивая сеть посредников: нотариус и переводчик зарегистрировали более 3 компаний нерезидентов",
			}
			results = append(results, makeNrRisk(
				jobID, "NR3",
				nr.FullName, nr.DirectorName, nr.BIN,
				nr.RegDate, float64(pair.Count), details,
			))
		}
	}

	log.Printf("[NR3] Найдено рисков: %d", len(results))
	return results
}

// ─── NR4: Финансовая пустышка ────────────────────────────────────────────────
// Уставной капитал < 100 000 ₸ — признак компании-однодневки.
// Минимальный УК для ТОО в РК — 100 тыс. тенге (для малого бизнеса).
func collectNR4(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	var nrRecords []models.NrRecord
	database.DB.Where("job_id = ? AND authorized_capital > 0 AND authorized_capital < 100000", jobID).
		Find(&nrRecords)

	for _, nr := range nrRecords {
		details := map[string]interface{}{
			"company_name":       nr.FullName,
			"bin":                nr.BIN,
			"director_name":      nr.DirectorName,
			"director_country":   nr.DirectorCountry,
			"authorized_capital": nr.AuthorizedCapital,
			"threshold":          100000,
			"activity_type":      nr.ActivityType,
			"legal_address":      nr.LegalAddress,
			"notary":             nr.Notary,
			"translator":         nr.Translator,
			"reason":             "Уставной капитал ниже минимального порога (100 000 ₸) — признак фиктивной компании",
		}
		results = append(results, makeNrRisk(
			jobID, "NR4",
			nr.FullName, nr.DirectorName, nr.BIN,
			nr.RegDate, 100000-nr.AuthorizedCapital, details,
		))
	}

	log.Printf("[NR4] Найдено рисков: %d", len(results))
	return results
}

// ─── NR5: Финансовая неактивность (Ответы БВУ) ──────────────────────────────
// Компания не имеет банковского счета (статус "Отсутствует") — невозможно вести
// хозяйственную деятельность. Или счёт "Открыт", но баланс < 10 000 ₸ —
// признак компании-однодневки ("спящий" счёт).
func collectNR5(jobID uint) []models.DetectedRisk {
	var results []models.DetectedRisk

	// --- Красный флаг: счёт отсутствует ---
	type noAccountRow struct {
		BIN               string
		CompanyName       string
		AccountStatus     string
		AuthorizedCapital float64
		RegDate           time.Time
		DirectorName      string
	}

	var noAccountRows []noAccountRow
	database.DB.Raw(`
		SELECT
			b.bin,
			b.company_name,
			b.status AS account_status,
			COALESCE(nr.authorized_capital, 0) AS authorized_capital,
			nr.reg_date,
			nr.director_name
		FROM bank_responses b
		LEFT JOIN nr_records nr ON nr.bin = b.bin AND nr.job_id = ?
		WHERE b.status = 'Отсутствует'
		   OR b.account_number = 'Отсутствует'
	`, jobID).Scan(&noAccountRows)

	for _, r := range noAccountRows {
		details := map[string]interface{}{
			"bin":                r.BIN,
			"company_name":       r.CompanyName,
			"director_name":      r.DirectorName,
			"account_status":     r.AccountStatus,
			"balance":            0,
			"authorized_capital": r.AuthorizedCapital,
			"reason":             "Банковский счёт отсутствует — компания не ведёт реальной деятельности",
		}

		riskDate := r.RegDate
		if riskDate.IsZero() {
			riskDate = time.Now()
		}

		results = append(results, makeNrRisk(
			jobID, "NR5",
			r.CompanyName, r.DirectorName, r.BIN,
			riskDate, r.AuthorizedCapital, details,
		))
	}

	// --- Жёлтый флаг: счёт открыт, но баланс подозрительно низкий ---
	type lowBalanceRow struct {
		BIN               string
		CompanyName       string
		AccountStatus     string
		Balance           float64
		AuthorizedCapital float64
		RegDate           time.Time
		DirectorName      string
	}

	var lowBalanceRows []lowBalanceRow
	database.DB.Raw(`
		SELECT
			b.bin,
			b.company_name,
			b.status AS account_status,
			b.balance,
			COALESCE(nr.authorized_capital, 0) AS authorized_capital,
			nr.reg_date,
			nr.director_name
		FROM bank_responses b
		LEFT JOIN nr_records nr ON nr.bin = b.bin AND nr.job_id = ?
		WHERE b.status ILIKE '%Открыт%'
		  AND b.balance < 10000
	`, jobID).Scan(&lowBalanceRows)

	for _, r := range lowBalanceRows {
		details := map[string]interface{}{
			"bin":                r.BIN,
			"company_name":       r.CompanyName,
			"director_name":      r.DirectorName,
			"account_status":     r.AccountStatus,
			"balance":            r.Balance,
			"authorized_capital": r.AuthorizedCapital,
			"reason":             "Подозрительно низкий баланс на счёте (менее 10 000 ₸) — признаки компании-однодневки",
		}

		riskDate := r.RegDate
		if riskDate.IsZero() {
			riskDate = time.Now()
		}

		results = append(results, makeNrRisk(
			jobID, "NR5",
			r.CompanyName, r.DirectorName, r.BIN,
			riskDate, r.AuthorizedCapital, details,
		))
	}

	log.Printf("[NR5] Найдено рисков: %d (без счёта: %d, низкий баланс: %d)",
		len(results), len(noAccountRows), len(lowBalanceRows))
	return results
}
