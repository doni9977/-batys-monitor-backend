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
	database.DB.Model(&models.BankResponse{}).Count(&count)
	fmt.Printf("Bank responses in DB: %d\n", count)
}
