package main
import (
	"fmt"
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/danialmarat/batys-monitor-backend/internal/services"
	"github.com/joho/godotenv"
)
func main() {
	godotenv.Load(".env")
	database.ConnectDb()
	
	var job models.RiskJob
	database.DB.Where("domain = ?", "nr").Order("created_at DESC").First(&job)
	fmt.Printf("Re-running NR engine for job %d...\n", job.ID)
	
	services.RunAllNrRiskEngines(job.ID)
	
	var count int64
	database.DB.Model(&models.DetectedRisk{}).Where("job_id = ? AND indicator = ?", job.ID, "NR5").Count(&count)
	fmt.Printf("Done! NR5 risks created: %d\n", count)
	
	database.DB.Model(&models.DetectedRisk{}).Where("job_id = ?", job.ID).Count(&count)
	fmt.Printf("Total NR risks for job: %d\n", count)
}
