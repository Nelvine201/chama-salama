package main

import (
	"database/sql"
	"fmt"
)

type Withdrawal struct {
	ID          int64
	RequestedBy int64
	Amount      float64
	Reason      string
	Status      string
}

func CreateWithdrawal(db *sql.DB, requestedBy, chamaID int64, amount float64, reason string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be greater than zero")
	}

	result, err := db.Exec(
		"INSERT INTO withdrawals (requested_by, chama_id, amount, reason, status) VALUES (?, ?, ?, ?, ?)",
		requestedBy, chamaID, amount, reason, "pending",
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
func ApproveWithdrawal(db *sql.DB, withdrawalID, memberID int64) (bool, error) {
	var alreadyApproved int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM withdrawal_approvals WHERE withdrawal_id = ? AND member_id = ?",
		withdrawalID, memberID,
	).Scan(&alreadyApproved)
	if err != nil {
		return false, err
	}
	if alreadyApproved > 0 {
		return false, fmt.Errorf("this member has already approved this withdrawal")
	}

	_, err = db.Exec(
		"INSERT INTO withdrawal_approvals (withdrawal_id, member_id) VALUES (?, ?)",
		withdrawalID, memberID,
	)
	if err != nil {
		return false, err
	}

	var approvalCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM withdrawal_approvals WHERE withdrawal_id = ?",
		withdrawalID,
	).Scan(&approvalCount)
	if err != nil {
		return false, err
	}

	var threshold int
	err = db.QueryRow(
		"SELECT COALESCE(gs.approval_threshold, 1) FROM withdrawals w LEFT JOIN group_settings gs ON gs.chama_id = w.chama_id WHERE w.id = ?",
		withdrawalID,
	).Scan(&threshold)
	if err != nil {
		return false, err
	}

	if approvalCount >= threshold {
		_, err = db.Exec("UPDATE withdrawals SET status = 'approved' WHERE id = ?", withdrawalID)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}
