package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS gts_listings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at INTEGER NOT NULL,
  pid INTEGER NOT NULL,
  generation INTEGER NOT NULL,
  game TEXT NOT NULL,
  species INTEGER NOT NULL, form INTEGER NOT NULL, level INTEGER NOT NULL, gender INTEGER NOT NULL,
  shiny INTEGER NOT NULL, nickname TEXT NOT NULL, ot TEXT NOT NULL,
  format TEXT NOT NULL, data BLOB NOT NULL,
  req_species INTEGER NOT NULL DEFAULT 0, req_gender INTEGER NOT NULL DEFAULT -1,
  req_level_min INTEGER NOT NULL DEFAULT 0, req_level_max INTEGER NOT NULL DEFAULT 100,
  status TEXT NOT NULL DEFAULT 'open',
  traded_at INTEGER, returned_format TEXT, returned_data BLOB
);
CREATE INDEX IF NOT EXISTS gts_open ON gts_listings(status, generation, species);
CREATE UNIQUE INDEX IF NOT EXISTS gts_one_per_pid ON gts_listings(pid, generation) WHERE status IN ('open','traded');
CREATE TABLE IF NOT EXISTS battle_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT, created_at INTEGER NOT NULL,
  generation INTEGER NOT NULL, game TEXT NOT NULL, format TEXT NOT NULL, season TEXT NOT NULL,
  pid INTEGER NOT NULL, result TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS battle_fs ON battle_records(format, season);
CREATE TABLE IF NOT EXISTS battle_members (
  record_id INTEGER NOT NULL REFERENCES battle_records(id),
  species INTEGER NOT NULL, form INTEGER NOT NULL, level INTEGER NOT NULL,
  ability INTEGER NOT NULL, item INTEGER NOT NULL, nature INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS members_rec ON battle_members(record_id);
CREATE TABLE IF NOT EXISTS battle_moves (record_id INTEGER NOT NULL, species INTEGER NOT NULL, move INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS moves_rec ON battle_moves(record_id);
`

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db}, nil
}

// Request is what the depositor wants back. Species 0 and Gender -1 mean any.
type Request struct {
	Species  int `json:"species"`
	Gender   int `json:"gender"`
	LevelMin int `json:"level_min"`
	LevelMax int `json:"level_max"`
}

type Listing struct {
	ID         int64   `json:"id"`
	CreatedAt  int64   `json:"created_at"`
	PID        int64   `json:"pid,omitempty"`
	Generation int     `json:"generation"`
	Game       string  `json:"game"`
	Species    int     `json:"species"`
	Form       int     `json:"form"`
	Level      int     `json:"level"`
	Gender     int     `json:"gender"`
	Shiny      bool    `json:"shiny"`
	Nickname   string  `json:"nickname"`
	OT         string  `json:"ot"`
	Request    Request `json:"request"`
	Status     string  `json:"status"`
	Format     string  `json:"format,omitempty"`
	Data       []byte  `json:"data,omitempty"`
	// Returned is the Pokemon the depositor received in exchange; set once traded.
	ReturnedFormat string `json:"returned_format,omitempty"`
	Returned       []byte `json:"returned,omitempty"`
}

var ErrConflict = errors.New("conflict")
var ErrNotFound = errors.New("not found")

func (s *Store) Deposit(ctx context.Context, pid int64, game string, v *Verdict, format string, data []byte, r Request) (*Listing, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `INSERT INTO gts_listings
	 (created_at,pid,generation,game,species,form,level,gender,shiny,nickname,ot,format,data,req_species,req_gender,req_level_min,req_level_max)
	 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		now, pid, v.Generation, game, v.Species, v.Form, v.Level, v.Gender, v.Shiny, v.Nickname, v.OT, format, data,
		r.Species, r.Gender, r.LevelMin, r.LevelMax)
	if err != nil {
		if isUnique(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, id, true)
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint")
}

const listingCols = `id,created_at,pid,generation,game,species,form,level,gender,shiny,nickname,ot,format,data,
 req_species,req_gender,req_level_min,req_level_max,status,COALESCE(returned_format,''),returned_data`

type scanner interface{ Scan(...any) error }

func scanListing(r scanner, withData bool) (*Listing, error) {
	var l Listing
	var shiny bool
	err := r.Scan(&l.ID, &l.CreatedAt, &l.PID, &l.Generation, &l.Game, &l.Species, &l.Form, &l.Level, &l.Gender, &shiny,
		&l.Nickname, &l.OT, &l.Format, &l.Data, &l.Request.Species, &l.Request.Gender, &l.Request.LevelMin, &l.Request.LevelMax,
		&l.Status, &l.ReturnedFormat, &l.Returned)
	if err != nil {
		return nil, err
	}
	l.Shiny = shiny
	if !withData {
		l.Data, l.Returned, l.Format, l.ReturnedFormat, l.PID = nil, nil, "", "", 0
	}
	return &l, nil
}

// Get returns one listing; internal callers (withData) also get the file and pid.
func (s *Store) Get(ctx context.Context, id int64, withData bool) (*Listing, error) {
	l, err := scanListing(s.db.QueryRowContext(ctx, `SELECT `+listingCols+` FROM gts_listings WHERE id=?`, id), withData)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return l, err
}

type Search struct {
	Generation, Species, Gender, LevelMin, LevelMax int // 0 / -1 mean any
	Limit, Offset                                   int
	WithData                                        bool
	ExcludePID                                      int64
}

func (s *Store) Search(ctx context.Context, f Search) ([]*Listing, int, error) {
	where, args := ` WHERE status='open'`, []any{}
	if f.Generation > 0 {
		where += ` AND generation=?`
		args = append(args, f.Generation)
	}
	if f.Species > 0 {
		where += ` AND species=?`
		args = append(args, f.Species)
	}
	if f.Gender >= 0 {
		where += ` AND gender=?`
		args = append(args, f.Gender)
	}
	if f.LevelMin > 0 {
		where += ` AND level>=?`
		args = append(args, f.LevelMin)
	}
	if f.LevelMax > 0 {
		where += ` AND level<=?`
		args = append(args, f.LevelMax)
	}
	if f.ExcludePID != 0 {
		where += ` AND pid<>?`
		args = append(args, f.ExcludePID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gts_listings`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+listingCols+` FROM gts_listings`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*Listing
	for rows.Next() {
		l, err := scanListing(rows, f.WithData)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// Mine is the depositor's own listing for a generation, if any.
func (s *Store) Mine(ctx context.Context, pid int64, gen int) (*Listing, error) {
	l, err := scanListing(s.db.QueryRowContext(ctx, `SELECT `+listingCols+` FROM gts_listings
	 WHERE pid=? AND generation=? AND status IN ('open','traded') ORDER BY id DESC LIMIT 1`, pid, gen), true)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return l, err
}

// Trade claims an open listing for the offered (already legality-checked) Pokemon. Exactly one
// caller wins the race.
func (s *Store) Trade(ctx context.Context, id, byPID int64, offered *Verdict, format string, data []byte) (*Listing, error) {
	l, err := s.Get(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if l.PID == byPID {
		return nil, ErrConflict
	}
	rq := l.Request
	if (rq.Species != 0 && rq.Species != offered.Species) ||
		(rq.Gender >= 0 && rq.Gender != offered.Gender) ||
		(rq.LevelMin > 0 && offered.Level < rq.LevelMin) ||
		(rq.LevelMax > 0 && offered.Level > rq.LevelMax) ||
		offered.Generation != l.Generation {
		return nil, errors.New("offered Pokemon does not match the request")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE gts_listings SET status='traded', traded_at=?, returned_format=?, returned_data=?
	 WHERE id=? AND status='open'`, time.Now().Unix(), format, data, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrConflict
	}
	return l, nil
}

// Finish ends a listing: withdraw an open one, or collect a traded one. Returns the row it closed.
func (s *Store) Finish(ctx context.Context, id, pid int64) (*Listing, error) {
	l, err := s.Get(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if l.PID != pid {
		return nil, ErrNotFound
	}
	next := map[string]string{"open": "withdrawn", "traded": "collected"}[l.Status]
	if next == "" {
		return nil, ErrConflict
	}
	res, err := s.db.ExecContext(ctx, `UPDATE gts_listings SET status=? WHERE id=? AND status=?`, next, id, l.Status)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrConflict
	}
	l.Status = next
	return l, nil
}

func (s *Store) RecordBattle(ctx context.Context, gen int, game, format, season string, pid int64, result string, team []*Verdict) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO battle_records(created_at,generation,game,format,season,pid,result) VALUES (?,?,?,?,?,?,?)`,
		time.Now().Unix(), gen, game, format, season, pid, result)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	for _, v := range team {
		if _, err := tx.ExecContext(ctx, `INSERT INTO battle_members VALUES (?,?,?,?,?,?,?)`, id, v.Species, v.Form, v.Level, v.Ability, v.HeldItem, v.Nature); err != nil {
			return 0, err
		}
		for _, m := range v.Moves {
			if _, err := tx.ExecContext(ctx, `INSERT INTO battle_moves VALUES (?,?,?)`, id, v.Species, m); err != nil {
				return 0, err
			}
		}
	}
	return id, tx.Commit()
}

type UsageRow struct {
	ID      int     `json:"id"`
	Species int     `json:"species,omitempty"`
	Uses    int     `json:"uses"`
	Pct     float64 `json:"usage_pct"`
	Wins    int     `json:"wins,omitempty"`
	Losses  int     `json:"losses,omitempty"`
	WinRate float64 `json:"win_rate,omitempty"`
}

type Showcase struct {
	Format    string     `json:"format"`
	Season    string     `json:"season"`
	Battles   int        `json:"battles"`
	Pokemon   []UsageRow `json:"pokemon"`
	Moves     []UsageRow `json:"moves"`
	Abilities []UsageRow `json:"abilities"`
	Items     []UsageRow `json:"items"`
}

// Showcase is the Global Showcase / usage ranking for a format and season ("" = all).
func (s *Store) Showcase(ctx context.Context, gen int, format, season string, limit int) (*Showcase, error) {
	f := ` FROM battle_records r WHERE 1=1`
	args := []any{}
	if gen > 0 {
		f += ` AND r.generation=?`
		args = append(args, gen)
	}
	if format != "" {
		f += ` AND r.format=?`
		args = append(args, format)
	}
	if season != "" {
		f += ` AND r.season=?`
		args = append(args, season)
	}
	out := &Showcase{Format: format, Season: season, Pokemon: []UsageRow{}, Moves: []UsageRow{}, Abilities: []UsageRow{}, Items: []UsageRow{}}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+f, args...).Scan(&out.Battles); err != nil {
		return nil, err
	}
	sub := `r.id IN (SELECT r.id` + f + `)`
	q := func(tbl, col string, withResult bool) ([]UsageRow, error) {
		query := `SELECT ` + col + ` AS k, COUNT(*) AS n, SUM(r.result='win'), SUM(r.result='loss') FROM ` + tbl + ` t JOIN battle_records r ON r.id=t.record_id WHERE ` + sub + ` GROUP BY k HAVING k IS NOT NULL ORDER BY n DESC, k LIMIT ?`
		rows, err := s.db.QueryContext(ctx, query, append(append([]any{}, args...), limit)...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		res := []UsageRow{}
		for rows.Next() {
			var u UsageRow
			if err := rows.Scan(&u.ID, &u.Uses, &u.Wins, &u.Losses); err != nil {
				return nil, err
			}
			if out.Battles > 0 {
				u.Pct = float64(u.Uses) / float64(out.Battles)
			}
			if d := u.Wins + u.Losses; d > 0 && withResult {
				u.WinRate = float64(u.Wins) / float64(d)
			}
			if !withResult {
				u.Wins, u.Losses = 0, 0
			}
			res = append(res, u)
		}
		return res, rows.Err()
	}
	var err error
	if out.Pokemon, err = q("battle_members", "t.species", true); err != nil {
		return nil, err
	}
	for i := range out.Pokemon {
		out.Pokemon[i].Species = out.Pokemon[i].ID
	}
	if out.Moves, err = q("battle_moves", "t.move", false); err != nil {
		return nil, err
	}
	if out.Abilities, err = q("battle_members", "t.ability", false); err != nil {
		return nil, err
	}
	if out.Items, err = q("battle_members", "NULLIF(t.item,0)", false); err != nil {
		return nil, err
	}
	return out, nil
}

type FormatInfo struct {
	Generation int      `json:"generation"`
	Format     string   `json:"format"`
	Seasons    []string `json:"seasons"`
	Battles    int      `json:"battles"`
}

func (s *Store) Formats(ctx context.Context) ([]FormatInfo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT generation, format, season, COUNT(*) FROM battle_records GROUP BY generation, format, season ORDER BY generation, format, season DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FormatInfo{}
	for rows.Next() {
		var g, n int
		var f, se string
		if err := rows.Scan(&g, &f, &se, &n); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Generation != g || out[len(out)-1].Format != f {
			out = append(out, FormatInfo{Generation: g, Format: f})
		}
		c := &out[len(out)-1]
		c.Seasons = append(c.Seasons, se)
		c.Battles += n
	}
	return out, rows.Err()
}

// Stats is the front-page counters.
func (s *Store) Stats(ctx context.Context) (map[string]int, error) {
	m := map[string]int{}
	for k, q := range map[string]string{
		"gts_open":   `SELECT COUNT(*) FROM gts_listings WHERE status='open'`,
		"gts_traded": `SELECT COUNT(*) FROM gts_listings WHERE status IN ('traded','collected')`,
		"battles":    `SELECT COUNT(*) FROM battle_records`,
	} {
		var n int
		if err := s.db.QueryRowContext(ctx, q).Scan(&n); err != nil {
			return nil, err
		}
		m[k] = n
	}
	return m, nil
}
