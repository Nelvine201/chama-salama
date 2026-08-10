package main
import (
	"database/sql"
	"fmt"
)

type Contribution struct {
	ID       int64
	MemberID int64
	Amount   float64
	PaidOn   string
	Source   string
	Status   string
}
func RecordContribution(db *sql.DB, memberID int64, amount float64, paidOn, source string) (int64,error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be greater than zero")
	}
	result, err :=db.Exec(
		"INSERT INTO contributions (member_id, amount, paid_on, source, status) VALUES (?, ?, ?, ?, ?)",
		memberID, amount, paidOn, source, "synced",
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil

}