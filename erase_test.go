package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEraseByPID(t *testing.T) {
	t.Setenv("ACCOUNT_ERASE_KEY", "erase-key-0123456789")
	st := testStore(t)
	h := (&API{Store: st, Check: fakeCheck{}, Key: "k"}).EraseRoutes()
	ctx := context.Background()
	v := &Verdict{Generation: 6, Species: 25, Level: 30, Moves: []int{33}}
	for _, pid := range []int64{1099511627776, 1099511627777} {
		if _, err := st.Deposit(ctx, pid, "x", v, "pk6", []byte{25, 30, 0}, Request{Gender: -1, LevelMax: 100}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.RecordBattle(ctx, 6, "x", "singles", "s1", pid, "win", []*Verdict{v}); err != nil {
			t.Fatal(err)
		}
	}
	del := func(auth, path string) int {
		r := httptest.NewRequest("DELETE", path, nil)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	const u = "/internal/accounts/11111111-1111-4111-8111-111111111111/data"
	if c := del("", u+"?pid=1099511627776"); c != http.StatusUnauthorized {
		t.Fatalf("no key %d", c)
	}
	if c := del("Bearer nope", u+"?pid=1099511627776"); c != http.StatusUnauthorized {
		t.Fatalf("wrong key %d", c)
	}
	if c := del("Bearer erase-key-0123456789", "/internal/accounts/x/data"); c != 400 {
		t.Fatalf("bad uuid %d", c)
	}
	count := func(pid int64) (n int) {
		st.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM gts_listings WHERE pid=?)+(SELECT count(*) FROM battle_records WHERE pid=?)`, pid, pid).Scan(&n)
		return
	}
	if del("Bearer erase-key-0123456789", u+"?pid=1099511627776") != 200 {
		t.Fatal("erase")
	}
	if count(1099511627776) != 0 || count(1099511627777) != 2 {
		t.Fatalf("after erase: %d / %d", count(1099511627776), count(1099511627777))
	}
	var members, moves int
	st.db.QueryRowContext(ctx, `SELECT count(*) FROM battle_members`).Scan(&members)
	st.db.QueryRowContext(ctx, `SELECT count(*) FROM battle_moves`).Scan(&moves)
	if members != 1 || moves != 1 {
		t.Fatalf("members/moves of other pid: %d %d", members, moves)
	}
	n, err := st.ErasePIDs(ctx, []int64{1099511627776})
	if err != nil || n != 0 {
		t.Fatalf("second run %d %v", n, err)
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "")
	if c := del("Bearer ", u); c != 503 {
		t.Fatalf("no env key %d", c)
	}
}
