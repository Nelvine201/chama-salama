package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SimulatedPayment struct {
	Phone    string
	Amount   float64
	ChamaID  int64
	MemberID int64
	Outcome  string
}

func SimulateMPesaPayment(db *sql.DB, payment SimulatedPayment) (int64, error) {
	if db == nil {
		return 0, errors.New("database is required")
	}

	payment.Phone = strings.TrimSpace(payment.Phone)
	payment.Outcome = strings.ToLower(strings.TrimSpace(payment.Outcome))

	if payment.Phone == "" {
		return 0, errors.New("phone number is required")
	}
	if payment.Amount <= 0 {
		return 0, errors.New("amount must be greater than zero")
	}
	if payment.ChamaID <= 0 || payment.MemberID <= 0 {
		return 0, errors.New("valid member and chama IDs are required")
	}

	switch payment.Outcome {
	case "success", "failed", "cancelled":
	default:
		return 0, fmt.Errorf("unsupported outcome %q: use success, failed, or cancelled", payment.Outcome)
	}

	status := "failed"
	if payment.Outcome == "success" {
		status = "synced"
	}

	result, err := db.Exec(
		`INSERT INTO contributions
			(member_id, chama_id, amount, paid_on, source, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		payment.MemberID,
		payment.ChamaID,
		payment.Amount,
		time.Now().Format("2006-01-02"),
		"mpesa_simulation:"+payment.Phone,
		status,
	)
	if err != nil {
		return 0, fmt.Errorf("record simulated payment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get simulated contribution ID: %w", err)
	}

	return id, nil
}
