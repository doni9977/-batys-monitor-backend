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
	var count int64
	database.DB.Model(&models.DetectedRisk{}).Where("indicator = ?", "NR5").Count(&count)
	fmt.Printf("NR5 risks in DB: %d\n", count)
}
