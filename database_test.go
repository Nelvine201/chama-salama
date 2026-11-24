package main

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteForeignKeysEnabledOnEveryConnection(t *testing.T) {
	db, err := sql.Open(
		"sqlite",
		"file:foreign_keys_test?mode=memory&cache=shared&_pragma=foreign_keys=on",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(2)

	conn1, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()

	conn2, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	for i, conn := range []*sql.Conn{conn1, conn2} {
		var enabled int

		err := conn.QueryRowContext(
			context.Background(),
			"PRAGMA foreign_keys",
		).Scan(&enabled)
		if err != nil {
			t.Fatal(err)
		}

		if enabled != 1 {
			t.Errorf("connection %d: expected foreign_keys=1, got %d", i+1, enabled)
		}
	}
}
