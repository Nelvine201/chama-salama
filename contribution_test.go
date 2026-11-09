package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupContributionDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
		CREATE TABLE contributions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			member_id INTEGER NOT NULL,
			chama_id INTEGER,
			cycle_id INTEGER,
			amount REAL NOT NULL,
			paid_on DATE NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			checkout_request_id TEXT,
			phone_number TEXT,
			mpesa_receipt_number TEXT
		)
	`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func insertPendingContribution(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO contributions (
			member_id, chama_id, cycle_id, amount, paid_on,
			source, status, checkout_request_id, phone_number
		)
		VALUES (1, 1, 1, 100, DATE('now'), 'mpesa',
		        'pending', 'ws_CO_test123', '0712345678')
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfirmSTKContribution(t *testing.T) {
	tests := []struct {
		name        string
		success     bool
		amount      float64
		phone       string
		receipt     string
		wantError   bool
		wantStatus  string
		wantReceipt string
	}{
		{
			name:        "valid callback",
			success:     true,
			amount:      100,
			phone:       "254712345678",
			receipt:     "QGH123ABC",
			wantStatus:  "synced",
			wantReceipt: "QGH123ABC",
		},
		{
			name:       "wrong amount",
			success:    true,
			amount:     200,
			phone:      "254712345678",
			receipt:    "QGH123ABC",
			wantError:  true,
			wantStatus: "pending",
		},
		{
			name:       "wrong phone",
			success:    true,
			amount:     100,
			phone:      "254722222222",
			receipt:    "QGH123ABC",
			wantError:  true,
			wantStatus: "pending",
		},
		{
			name:       "missing receipt",
			success:    true,
			amount:     100,
			phone:      "254712345678",
			wantError:  true,
			wantStatus: "pending",
		},
		{
			name:       "failed payment",
			success:    false,
			wantStatus: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContributionDB(t)
			insertPendingContribution(t, db)

			err := ConfirmSTKContribution(
				db,
				"ws_CO_test123",
				tt.success,
				tt.amount,
				tt.phone,
				tt.receipt,
			)

			if tt.wantError && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var status, receipt string
			err = db.QueryRow(`
				SELECT status, COALESCE(mpesa_receipt_number, '')
				FROM contributions
				WHERE checkout_request_id = ?
			`, "ws_CO_test123").Scan(&status, &receipt)
			if err != nil {
				t.Fatal(err)
			}

			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}

			if receipt != tt.wantReceipt {
				t.Errorf("receipt = %q, want %q", receipt, tt.wantReceipt)
			}
		})
	}
}
