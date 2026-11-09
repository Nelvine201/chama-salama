package main

import (
	"database/sql"
	"fmt"
	"strings"
)

type Contribution struct {
	ID       int64
	MemberID int64
	Amount   float64
	PaidOn   string
	Source   string
	Status   string
}

func RecordContribution(db *sql.DB, memberID int64, amount float64, paidOn, source string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be greater than zero")
	}
	result, err := db.Exec(
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

func RecordContributionOffline(db *sql.DB, memberID int64, amount float64, paidOn, source string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be greater than zero")
	}

	result, err := db.Exec(
		"INSERT INTO contributions (member_id, amount, paid_on, source, status) VALUES (?, ?, ?, ?, ?)",
		memberID, amount, paidOn, source, "pending",
	)
	if err != nil {
		return 0, err
	}

	contributionID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	_, err = db.Exec(
		"INSERT INTO sync_queue (contribution_id, sync_status) VALUES (?, ?)",
		contributionID, "pending",
	)
	if err != nil {
		return 0, err
	}

	return contributionID, nil
}

func ProcessSyncQueue(db *sql.DB) (int, error) {
	rows, err := db.Query("SELECT id, contribution_id FROM sync_queue WHERE sync_status = 'pending'")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var queueItems []struct {
		QueueID        int64
		ContributionID int64
	}

	for rows.Next() {
		var item struct {
			QueueID        int64
			ContributionID int64
		}
		if err := rows.Scan(&item.QueueID, &item.ContributionID); err != nil {
			return 0, err
		}
		queueItems = append(queueItems, item)
	}

	synced := 0
	for _, item := range queueItems {
		_, err := db.Exec("UPDATE contributions SET status = 'synced' WHERE id = ?", item.ContributionID)
		if err != nil {
			return synced, err
		}

		_, err = db.Exec("UPDATE sync_queue SET sync_status = 'synced' WHERE id = ?", item.QueueID)
		if err != nil {
			return synced, err
		}

		synced++
	}

	return synced, nil
}

func GetGroupBalance(db *sql.DB) (float64, error) {
	var total float64
	err := db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM contributions WHERE status = 'synced'").Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

type ContributionWithMember struct {
	ID         int64
	MemberName string
	Amount     float64
	PaidOn     string
	Status     string
}

func GetRecentContributions(db *sql.DB, limit int) ([]ContributionWithMember, error) {
	rows, err := db.Query(
		`SELECT c.id, m.name, c.amount, c.paid_on, c.status
		 FROM contributions c
		 JOIN members m ON c.member_id = m.id
		 ORDER BY c.id DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContributionWithMember
	for rows.Next() {
		var c ContributionWithMember
		err := rows.Scan(&c.ID, &c.MemberName, &c.Amount, &c.PaidOn, &c.Status)
		if err != nil {
			return nil, err
		}
		results = append(results, c)
	}

	return results, nil
}

func RecordPendingSTKContribution(
	db *sql.DB,
	memberID, chamaID, cycleID int64,
	amount float64,
	phone string,
	checkoutRequestID string,
) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO contributions
    (member_id, chama_id, cycle_id, amount, paid_on, source, status, checkout_request_id, phone_number)
    VALUES (?, ?, ?, ?, DATE('now'), ?, ?, ?, ?)`,
		memberID,
		chamaID,
		cycleID,
		amount,
		"mpesa",
		"pending",
		checkoutRequestID,
		phone,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func ConfirmSTKContribution(
	db *sql.DB,
	checkoutRequestID string,
	success bool,
	amount float64,
	phone string,
	receipt string,
) error {
	if strings.TrimSpace(checkoutRequestID) == "" {
		return fmt.Errorf("checkout request ID is required")
	}

	if !success {
		result, err := db.Exec(`
            UPDATE contributions
            SET status = 'failed'
            WHERE checkout_request_id = ?
            AND status = 'pending'
        `, checkoutRequestID)
		if err != nil {
			return err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return fmt.Errorf("no pending contribution found for checkout request")
		}

		return nil
	}

	if amount <= 0 {
		return fmt.Errorf("invalid payment amount")
	}

	receipt = strings.TrimSpace(receipt)
	if receipt == "" {
		return fmt.Errorf("M-Pesa receipt number is required")
	}

	callbackPhone, valid := normalizeKenyanPhone(phone)
	if !valid {
		return fmt.Errorf("invalid callback phone number")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var expectedAmount float64
	var expectedPhone string

	err = tx.QueryRow(`
        SELECT amount, phone_number
        FROM contributions
        WHERE checkout_request_id = ?
        AND status = 'pending'
    `, checkoutRequestID).Scan(&expectedAmount, &expectedPhone)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no pending contribution found for checkout request")
		}
		return err
	}

	if amount != expectedAmount {
		return fmt.Errorf("payment amount does not match pending contribution")
	}

	storedPhone, valid := normalizeKenyanPhone(expectedPhone)
	if !valid || callbackPhone != storedPhone {
		return fmt.Errorf("payment phone number does not match pending contribution")
	}

	result, err := tx.Exec(`
        UPDATE contributions
        SET status = 'synced',
            mpesa_receipt_number = ?
        WHERE checkout_request_id = ?
        AND status = 'pending'
    `, receipt, checkoutRequestID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("contribution was not updated")
	}

	return tx.Commit()
}
