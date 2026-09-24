package main
import (
	"fmt"
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/joho/godotenv"
)
func main() {
	godotenv.Load(".env")
	database.ConnectDb()
	var job models.RiskJob
	err := database.DB.Where("domain = ?", "nr").Order("created_at DESC").First(&job).Error
	if err != nil {
		fmt.Println("No NR job found:", err)
		return
	}
	var count int64
	database.DB.Model(&models.DetectedRisk{}).Where("job_id = ?", job.ID).Count(&count)
	fmt.Printf("Latest job %d for NR has %d risks\n", job.ID, count)
	
	type Result struct { ClinicName string }
	var res []Result
	database.DB.Model(&models.DetectedRisk{}).Select("clinic_name").Where("job_id = ?", job.ID).Group("clinic_name").Find(&res)
	fmt.Printf("Unique clinic_names (subjects): %d\n", len(res))
}
