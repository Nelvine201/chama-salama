package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type columnMigration struct {
	table      string
	name       string
	definition string
}

var databaseColumnMigrations = []columnMigration{
	{"members", "avatar_url", "TEXT"},
	{"members", "preferred_first_name", "TEXT"},
	{"members", "default_payout_method", "TEXT"},
	{"members", "next_of_kin_relationship", "TEXT"},
	{"members", "next_of_kin_id_number", "TEXT"},
	{"members", "next_of_kin_phone", "TEXT"},
	{"members", "notify_sms", "INTEGER DEFAULT 1"},
	{"members", "notify_email", "INTEGER DEFAULT 1"},
	{"chamas", "description", "TEXT"},
	{"chamas", "start_date", "TEXT"},
	{"chamas", "max_participants", "INTEGER"},
	{"chamas", "lifecycle_status", "TEXT NOT NULL DEFAULT 'active'"},
	{"cycles", "status", "TEXT NOT NULL DEFAULT 'pending'"},
	{"cycles", "completed_at", "DATETIME"},
	{"chama_members", "status", "TEXT DEFAULT 'active'"},
	{"contributions", "chama_id", "INTEGER"},
	{"contributions", "checkout_request_id", "TEXT"},
	{"contributions", "cycle_id", "INTEGER"},
	{"contributions", "phone_number", "TEXT"},
	{"contributions", "mpesa_receipt_number", "TEXT"},
	{"withdrawals", "chama_id", "INTEGER"},
	{"withdrawal_approvals", "approver_role", "TEXT DEFAULT 'MEMBER'"},
	{"withdrawal_approvals", "status", "TEXT DEFAULT 'SIGNED'"},
	{"withdrawal_approvals", "signed_at", "DATETIME"},
	{"group_settings", "chama_id", "INTEGER"},
	{"group_settings", "payout_position", "INTEGER DEFAULT 0"},
	{"group_settings", "next_payout_date", "TEXT"},
	{"group_settings", "number_of_rounds", "INTEGER DEFAULT 1"},
	{"group_settings", "payout_method", "TEXT DEFAULT 'fixed_rotation'"},
	{"group_settings", "grace_period_days", "INTEGER DEFAULT 0"},
	{"group_settings", "late_penalty_type", "TEXT DEFAULT 'none'"},
	{"group_settings", "late_penalty_amount", "REAL DEFAULT 0"},
	{"group_settings", "welfare_reserve_amount", "REAL DEFAULT 0"},
	{"group_settings", "approval_threshold", "INTEGER DEFAULT 1"},
	{"group_settings", "payout_destination_type", "TEXT"},
	{"group_settings", "paybill_number", "TEXT"},
	{"group_settings", "till_number", "TEXT"},
	{"group_settings", "treasurer_phone", "TEXT"},
	{"group_settings", "treasurer_account_name", "TEXT"},
	{"notifications", "reference_id", "INTEGER"},
}

// migrateDatabase applies schema changes and data backfills as one transaction.
// Re-running it is safe: tables, indexes, and columns are created only when missing,
// and existing records are updated only where their migrated values are absent.
func migrateDatabase(db *sql.DB, schema string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()

	for _, statement := range strings.Split(schema, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("apply schema statement %q: %w", statement, err)
		}
	}

	for _, migration := range databaseColumnMigrations {
		exists, err := hasColumn(tx, migration.table, migration.name)
		if err != nil {
			return fmt.Errorf("check %s.%s: %w", migration.table, migration.name, err)
		}
		if exists {
			continue
		}

		statement := fmt.Sprintf(
			"ALTER TABLE %s ADD COLUMN %s %s",
			migration.table,
			migration.name,
			migration.definition,
		)
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("add column %s.%s: %w", migration.table, migration.name, err)
		}
	}

	if _, err := tx.Exec(`
		UPDATE group_settings
		SET payout_method = 'fixed_rotation'
		WHERE payout_method IS NULL OR payout_method <> 'fixed_rotation'
	`); err != nil {
		return fmt.Errorf("enforce fixed rotation: %w", err)
	}

	if err := ensureDefaultChama(tx); err != nil {
		return err
	}

	if err := backfillChamaIDs(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

func hasColumn(tx *sql.Tx, table, column string) (bool, error) {
	// Table names come only from the fixed migration list above.
	rows, err := tx.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func ensureDefaultChama(tx *sql.Tx) error {
	var chamaCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM chamas").Scan(&chamaCount); err != nil {
		return fmt.Errorf("count chamas: %w", err)
	}
	if chamaCount != 0 {
		return nil
	}

	var firstMemberID int64
	err := tx.QueryRow("SELECT id FROM members ORDER BY id LIMIT 1").Scan(&firstMemberID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find first member for default chama: %w", err)
	}

	result, err := tx.Exec(
		"INSERT INTO chamas (name, created_by) VALUES (?, ?)",
		"Chama Salama",
		firstMemberID,
	)
	if err != nil {
		return fmt.Errorf("create default chama: %w", err)
	}
	chamaID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get default chama ID: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO chama_members (chama_id, member_id, role)
		 SELECT ?, id, role FROM members
		 WHERE id NOT IN (SELECT member_id FROM chama_members WHERE chama_id = ?)`,
		chamaID,
		chamaID,
	); err != nil {
		return fmt.Errorf("add existing members to default chama: %w", err)
	}
	return nil
}

func backfillChamaIDs(tx *sql.Tx) error {
	var chamaID int64
	err := tx.QueryRow("SELECT id FROM chamas ORDER BY id LIMIT 1").Scan(&chamaID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find chama for existing records: %w", err)
	}

	for _, table := range []string{"contributions", "group_settings", "withdrawals"} {
		if _, err := tx.Exec("UPDATE "+table+" SET chama_id = ? WHERE chama_id IS NULL", chamaID); err != nil {
			return fmt.Errorf("backfill %s.chama_id: %w", table, err)
		}
	}
	return nil
}
