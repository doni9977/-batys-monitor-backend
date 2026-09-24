package main
import (
	"fmt"
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/joho/godotenv"
)
func main() {
	godotenv.Load(".env")
	database.ConnectDb()
	type countRow struct {
		Status string
		Count int
	}
	var res []countRow
	database.DB.Raw("SELECT status, COUNT(*) as count FROM bank_responses GROUP BY status").Scan(&res)
	for _, r := range res {
		fmt.Printf("Status: '%s', Count: %d\n", r.Status, r.Count)
	}
}
