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


