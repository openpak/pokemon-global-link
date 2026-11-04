package main

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ErasePIDs deletes every GTS listing and battle record of the given trainer ids (card #120).
// Global-link stores no account id: a trainer id is the pid HOME hands out per account, so the
// caller (pokemon-home) names the pids. Idempotent: a second run finds nothing and returns 0.
func (s *Store) ErasePIDs(ctx context.Context, pids []int64) (int, error) {
	if len(pids) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows := 0
	for _, pid := range pids {
		for _, q := range []string{
			`DELETE FROM battle_moves WHERE record_id IN (SELECT id FROM battle_records WHERE pid=?)`,
			`DELETE FROM battle_members WHERE record_id IN (SELECT id FROM battle_records WHERE pid=?)`,
			`DELETE FROM battle_records WHERE pid=?`,
			`DELETE FROM gts_listings WHERE pid=?`,
		} {
			res, err := tx.ExecContext(ctx, q, pid)
			if err != nil {
				return 0, err
			}
			n, _ := res.RowsAffected()
			rows += int(n)
		}
	}
	return rows, tx.Commit()
}

// eraseAccount serves DELETE /internal/accounts/{account_id}/data?pid=N (Bearer ACCOUNT_ERASE_KEY).
// Unset key = 503. Without a pid nothing in this store can be tied to the account, so it deletes 0 rows.
func (a *API) eraseAccount(w http.ResponseWriter, r *http.Request) {
	key := os.Getenv("ACCOUNT_ERASE_KEY")
	if key == "" {
		fail(w, 503, "erase disabled")
		return
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(key)) != 1 {
		fail(w, 401, "unauthorized")
		return
	}
	id := r.PathValue("account_id")
	if !uuidRE.MatchString(id) {
		fail(w, 400, "bad account id")
		return
	}
	var pids []int64
	for _, v := range r.URL.Query()["pid"] {
		p, err := strconv.ParseInt(v, 10, 64)
		if err != nil || p <= 0 {
			fail(w, 400, "bad pid")
			return
		}
		pids = append(pids, p)
	}
	ctx, c := ctxOf(r)
	defer c()
	n, err := a.Store.ErasePIDs(ctx, pids)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	log.Printf("account erased account=%s rows=%d service=pokemon-global-link", strings.ToLower(id), n)
	reply(w, 200, map[string]any{"service": "pokemon-global-link", "rows": n})
}

// EraseRoutes is the erase listener's handler (ERASE_ADDR, container network only: the public
// listener carries CORS-open read routes and must not carry this).
func (a *API) EraseRoutes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("DELETE /internal/accounts/{account_id}/data", a.eraseAccount)
	return m
}
