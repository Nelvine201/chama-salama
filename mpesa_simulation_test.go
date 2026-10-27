package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupSimulationDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE contributions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			member_id INTEGER NOT NULL,
			chama_id INTEGER,
			amount REAL NOT NULL,
			paid_on DATE NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestSimulateMPesaPaymentOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		outcome string
		want    string
	}{
		{"successful payment", "success", "synced"},
		{"failed payment", "failed", "failed"},
		{"cancelled payment", "cancelled", "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupSimulationDB(t)

			id, err := SimulateMPesaPayment(db, SimulatedPayment{
				Phone:    "0712345678",
				Amount:   100,
				ChamaID:  1,
				MemberID: 1,
				Outcome:  tt.outcome,
			})
			if err != nil {
				t.Fatalf("SimulateMPesaPayment() error = %v", err)
			}

			var status, source string
			err = db.QueryRow(
				"SELECT status, source FROM contributions WHERE id = ?",
				id,
			).Scan(&status, &source)
			if err != nil {
				t.Fatal(err)
			}

			if status != tt.want {
				t.Errorf("status = %q, want %q", status, tt.want)
			}
			if source != "mpesa_simulation:0712345678" {
				t.Errorf("source = %q", source)
			}
		})
	}
}

func TestSimulateMPesaPaymentRejectsInvalidInput(t *testing.T) {
	db := setupSimulationDB(t)

	tests := []SimulatedPayment{
		{Phone: "", Amount: 100, ChamaID: 1, MemberID: 1, Outcome: "success"},
		{Phone: "0712345678", Amount: 0, ChamaID: 1, MemberID: 1, Outcome: "success"},
		{Phone: "0712345678", Amount: 100, ChamaID: 1, MemberID: 1, Outcome: "unknown"},
	}

	for _, payment := range tests {
		if _, err := SimulateMPesaPayment(db, payment); err == nil {
			t.Errorf("expected error for payment %+v", payment)
		}
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM contributions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("invalid payments inserted %d records; want 0", count)
	}
}
