package models

// BankResponse хранит ответ БВУ (Банка Второго Уровня) по конкретной компании.
// Источник: Excel «bank_responses_mock.xlsx» или реальный ответ банка.
type BankResponse struct {
	ID            uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	BIN           string  `gorm:"type:varchar(20);index" json:"bin"`
	CompanyName   string  `gorm:"type:varchar(500)" json:"company_name"`
	AccountNumber string  `gorm:"type:varchar(100)" json:"account_number"`
	Currency      string  `gorm:"type:varchar(10)" json:"currency"`
	AccountType   string  `gorm:"type:varchar(100)" json:"account_type"`
	Status        string  `gorm:"type:varchar(100);index" json:"status"`
	Balance       float64 `gorm:"type:numeric(18,2);default:0" json:"balance"`
}

func (BankResponse) TableName() string {
	return "bank_responses"
}
