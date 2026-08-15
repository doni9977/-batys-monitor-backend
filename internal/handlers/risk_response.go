package handlers

import (
	"encoding/json"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
)

func riskToJSON(r models.DetectedRisk) map[string]interface{} {
	var details map[string]interface{}
	if len(r.Details) > 0 {
		_ = json.Unmarshal(r.Details, &details)
	}

	return map[string]interface{}{
		"id":          r.ID,
		"job_id":      r.JobID,
		"indicator":   r.Indicator,
		"clinic_name": r.ClinicName,
		"doctor_name": r.DoctorName,
		"patient_iin": r.PatientIIN,
		"risk_date":   r.RiskDate.Format("2006-01-02"),
		"amount":      r.Amount,
		"details":     details,
	}
}
