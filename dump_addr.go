package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	database.ConnectDb()

	var records []models.NrRecord
	database.DB.Select("full_name, legal_address").Find(&records)

	file, _ := os.Create("addresses.json")
	defer file.Close()
	json.NewEncoder(file).Encode(records)
	log.Printf("Dumped %d addresses", len(records))
}
