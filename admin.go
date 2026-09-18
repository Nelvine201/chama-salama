package main

import (
	"database/sql"
	"fmt"
)

type GroupSettings struct {
	ID                 int64
	ContributionAmount float64
	Frequency          string
	PayoutOrder        sql.NullString
}

func SetGroupSettings(db *sql.DB, amount float64, frequency string) error {
	if amount <= 0 {
		return fmt.Errorf("contribution amount must be greater than zero")
	}
	if frequency != "daily" && frequency != "weekly" && frequency != "monthly" {
		return fmt.Errorf("frequency must be daily, weekly, or monthly")
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM group_settings").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		_, err = db.Exec(
			"INSERT INTO group_settings (contribution_amount, frequency) VALUES (?, ?)",
			amount, frequency,
		)
	} else {
		_, err = db.Exec(
			"UPDATE group_settings SET contribution_amount = ?, frequency = ? WHERE id = (SELECT id FROM group_settings LIMIT 1)",
			amount, frequency,
		)
	}

	return err
}

func GetGroupSettings(db *sql.DB) (*GroupSettings, error) {
	var s GroupSettings
	err := db.QueryRow("SELECT id, contribution_amount, frequency, payout_order FROM group_settings LIMIT 1").
		Scan(&s.ID, &s.ContributionAmount, &s.Frequency, &s.PayoutOrder)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
