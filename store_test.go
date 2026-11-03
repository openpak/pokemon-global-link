package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// testDSN returns a DSN confined to a fresh schema of PGL_TEST_DATABASE_URL (skips when unset).
func testDSN(t *testing.T) string {
	base := os.Getenv("PGL_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("PGL_TEST_DATABASE_URL not set")
	}
	name := fmt.Sprintf("pgltest_%d", time.Now().UnixNano())
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE SCHEMA " + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + name + " CASCADE"); admin.Close() })
	return base + "&search_path=" + name
}

func testStore(t *testing.T) *Store {
	st, err := OpenStore(testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.db.Close() })
	return st
}

func TestOpenStoreNeedsDSN(t *testing.T) {
	if _, err := OpenStore(""); err == nil {
		t.Fatal("empty DSN must fail")
	}
}

func TestRestartKeepsState(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	a, err := OpenStore(dsn)
	if err != nil {
		t.Fatal(err)
	}
	v := &Verdict{Generation: 6, Species: 25, Level: 30, Moves: []int{33}}
	if _, err := a.Deposit(ctx, 1099511627776, "x", v, "pk6", []byte{25, 30, 0}, Request{Gender: -1, LevelMax: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RecordBattle(ctx, 6, "x", "singles", "s1", 1099511627776, "win", []*Verdict{v}); err != nil {
		t.Fatal(err)
	}
	a.db.Close()
	b, err := OpenStore(dsn) // a new instance on the same database
	if err != nil {
		t.Fatal(err)
	}
	defer b.db.Close()
	m, err := b.Stats(ctx)
	if err != nil || m["gts_open"] != 1 || m["battles"] != 1 {
		t.Fatalf("stats after restart: %v %v", m, err)
	}
	if _, err := b.Mine(ctx, 1099511627776, 6); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Deposit(ctx, 1099511627776, "x", v, "pk6", []byte{1}, Request{}); err != ErrConflict {
		t.Fatalf("second deposit: %v", err)
	}
	sc, err := b.Showcase(ctx, 6, "singles", "s1", 10)
	if err != nil || sc.Battles != 1 || len(sc.Pokemon) != 1 || len(sc.Moves) != 1 || sc.Pokemon[0].Wins != 1 {
		t.Fatalf("showcase: %+v %v", sc, err)
	}
}
