package main
import (
	"fmt"
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/joho/godotenv"
)
func main() {
	godotenv.Load(".env")
	database.ConnectDb()
	
	type Result struct {
		B_Bin string
		Nr_Bin string
	}
	var res []Result
	database.DB.Raw(`
		SELECT b.bin as b_bin, nr.bin as nr_bin
		FROM bank_responses b
		LEFT JOIN nr_records nr ON b.bin = nr.bin
		LIMIT 10
	`).Scan(&res)
	for _, r := range res {
		fmt.Printf("Bank BIN: '%s', NR BIN: '%s'\n", r.B_Bin, r.Nr_Bin)
	}
}
