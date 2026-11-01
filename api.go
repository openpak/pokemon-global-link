package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	emit "github.com/openpak/rewards-emit"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxData = 4096

type API struct {
	Store *Store
	Check Checker
	Key   string
	// Rewards reports completed trades to the rewards service; nil or disabled does nothing.
	Rewards *emit.Client
}

func (a *API) Routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })

	// Public, read-only, no trainer ids, no Pokemon files.
	m.HandleFunc("GET /api/v1/stats", a.stats)
	m.HandleFunc("GET /api/v1/gts/listings", a.listPublic)
	m.HandleFunc("GET /api/v1/gts/listings/{id}", a.getPublic)
	m.HandleFunc("GET /api/v1/battle/formats", a.formats)
	m.HandleFunc("GET /api/v1/showcase", a.showcase)

	// Internal, for the game-facing protocol handlers. Needs X-Internal-Key.
	m.HandleFunc("POST /internal/v1/gts/deposit", a.internal(a.deposit))
	m.HandleFunc("GET /internal/v1/gts/search", a.internal(a.searchInternal))
	m.HandleFunc("GET /internal/v1/gts/mine", a.internal(a.mine))
	m.HandleFunc("POST /internal/v1/gts/{id}/trade", a.internal(a.trade))
	m.HandleFunc("POST /internal/v1/gts/{id}/finish", a.internal(a.finish))
	m.HandleFunc("POST /internal/v1/battle/record", a.internal(a.record))
	return cors(m)
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Cache-Control", "public, max-age=15")
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		h.ServeHTTP(w, r)
	})
}

func (a *API) internal(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Key == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Key")), []byte(a.Key)) != 1 {
			fail(w, 401, "unauthorized")
			return
		}
		h(w, r)
	}
}

func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	reply(w, code, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, 400, "bad json")
		return false
	}
	return true
}

func ctxOf(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 20*time.Second)
}

func qint(r *http.Request, k string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(k)); err == nil {
		return v
	}
	return def
}

func page(r *http.Request) (limit, offset int) {
	limit, offset = qint(r, "limit", 50), qint(r, "offset", 0)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return
}

func (a *API) searchFilter(r *http.Request) Search {
	limit, offset := page(r)
	return Search{Generation: qint(r, "generation", 0), Species: qint(r, "species", 0), Gender: qint(r, "gender", -1),
		LevelMin: qint(r, "level_min", 0), LevelMax: qint(r, "level_max", 0), Limit: limit, Offset: offset}
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	m, err := a.Store.Stats(ctx)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	reply(w, 200, m)
}

func (a *API) listPublic(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	f := a.searchFilter(r)
	ls, total, err := a.Store.Search(ctx, f)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	if ls == nil {
		ls = []*Listing{}
	}
	reply(w, 200, map[string]any{"total": total, "limit": f.Limit, "offset": f.Offset, "listings": ls})
}

func (a *API) getPublic(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	l, err := a.Store.Get(ctx, id, false)
	if err != nil || l.Status == "withdrawn" || l.Status == "collected" {
		fail(w, 404, "not found")
		return
	}
	reply(w, 200, l)
}

func (a *API) formats(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	f, err := a.Store.Formats(ctx)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	reply(w, 200, map[string]any{"formats": f})
}

func (a *API) showcase(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	limit := qint(r, "limit", 30)
	if limit < 1 || limit > 200 {
		limit = 30
	}
	s, err := a.Store.Showcase(ctx, qint(r, "generation", 0), r.URL.Query().Get("format"), r.URL.Query().Get("season"), limit)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	reply(w, 200, s)
}

// --- internal ---

type pokemonIn struct {
	Format string `json:"format"` // pk4, pk5, pk6, ...
	Data   []byte `json:"data"`   // base64 in JSON
}

// check runs the legality service. Illegal or unreachable is a refusal; nothing is stored.
func (a *API) check(ctx context.Context, p pokemonIn) (*Verdict, int, string) {
	if len(p.Data) == 0 || len(p.Data) > maxData {
		return nil, 400, "data missing or too large"
	}
	v, err := a.Check.Check(ctx, p.Data, p.Format)
	if err != nil {
		if v != nil { // the service answered: not a Pokemon file
			return v, 422, err.Error()
		}
		return nil, 503, "legality service unavailable"
	}
	if !v.Valid {
		return v, 422, "illegal Pokemon"
	}
	return v, 0, ""
}

func (a *API) deposit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PID     int64   `json:"pid"`
		Game    string  `json:"game"`
		Request Request `json:"request"`
		pokemonIn
	}
	if !decode(w, r, &in) {
		return
	}
	if in.PID == 0 || in.Game == "" {
		fail(w, 400, "pid and game required")
		return
	}
	if in.Request.LevelMax == 0 {
		in.Request.LevelMax = 100
	}
	ctx, c := ctxOf(r)
	defer c()
	v, code, msg := a.check(ctx, in.pokemonIn)
	if code != 0 {
		reply(w, code, map[string]any{"error": msg, "verdict": v})
		return
	}
	l, err := a.Store.Deposit(ctx, in.PID, in.Game, v, in.Format, in.Data, in.Request)
	switch {
	case errors.Is(err, ErrConflict):
		fail(w, 409, "this trainer already has a Pokemon in the GTS")
	case err != nil:
		fail(w, 500, "store")
	default:
		reply(w, 201, l)
	}
}

func (a *API) searchInternal(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	f := a.searchFilter(r)
	f.WithData = true
	f.ExcludePID = int64(qint(r, "exclude_pid", 0))
	ls, total, err := a.Store.Search(ctx, f)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	if ls == nil {
		ls = []*Listing{}
	}
	reply(w, 200, map[string]any{"total": total, "listings": ls})
}

func (a *API) mine(w http.ResponseWriter, r *http.Request) {
	ctx, c := ctxOf(r)
	defer c()
	l, err := a.Store.Mine(ctx, int64(qint(r, "pid", 0)), qint(r, "generation", 0))
	if errors.Is(err, ErrNotFound) {
		fail(w, 404, "none")
		return
	} else if err != nil {
		fail(w, 500, "store")
		return
	}
	reply(w, 200, l)
}

func (a *API) trade(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PID int64 `json:"pid"`
		pokemonIn
	}
	if !decode(w, r, &in) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	ctx, c := ctxOf(r)
	defer c()
	v, code, msg := a.check(ctx, in.pokemonIn)
	if code != 0 {
		reply(w, code, map[string]any{"error": msg, "verdict": v})
		return
	}
	l, err := a.Store.Trade(ctx, id, in.PID, v, in.Format, in.Data)
	switch {
	case errors.Is(err, ErrNotFound):
		fail(w, 404, "not found")
	case errors.Is(err, ErrConflict):
		fail(w, 409, "listing is gone or yours")
	case err != nil:
		fail(w, 422, err.Error())
	default:
		a.tradeDone(l.ID, "taker", l.Generation, in.PID)
		reply(w, 200, l)
	}
}

func (a *API) finish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PID int64 `json:"pid"`
	}
	if !decode(w, r, &in) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	ctx, c := ctxOf(r)
	defer c()
	l, err := a.Store.Finish(ctx, id, in.PID)
	switch {
	case errors.Is(err, ErrNotFound):
		fail(w, 404, "not found")
	case errors.Is(err, ErrConflict):
		fail(w, 409, "nothing to finish")
	case err != nil:
		fail(w, 500, "store")
	default:
		if l.Status == "collected" { // collecting a traded listing completes the trade for its owner
			a.tradeDone(l.ID, "owner", l.Generation, l.PID)
		}
		reply(w, 200, l)
	}
}

func (a *API) record(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PID        int64       `json:"pid"`
		Generation int         `json:"generation"`
		Game       string      `json:"game"`
		Format     string      `json:"format"`
		Season     string      `json:"season"`
		Result     string      `json:"result"`
		Team       []pokemonIn `json:"team"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.PID == 0 || in.Format == "" || in.Generation == 0 || len(in.Team) == 0 || len(in.Team) > 6 ||
		(in.Result != "win" && in.Result != "loss" && in.Result != "draw") {
		fail(w, 400, "pid, generation, format, result (win|loss|draw) and a team of 1-6 required")
		return
	}
	if in.Season == "" {
		in.Season = time.Now().UTC().Format("2006-01")
	}
	ctx, c := ctxOf(r)
	defer c()
	vs := make([]*Verdict, 0, len(in.Team))
	verdicts := make([]*Verdict, len(in.Team))
	for i, p := range in.Team {
		v, code, msg := a.check(ctx, p)
		verdicts[i] = v
		if code != 0 {
			reply(w, code, map[string]any{"error": msg, "member": i, "verdicts": verdicts})
			return
		}
		vs = append(vs, v)
	}
	id, err := a.Store.RecordBattle(ctx, in.Generation, in.Game, in.Format, in.Season, in.PID, in.Result, vs)
	if err != nil {
		fail(w, 500, "store")
		return
	}
	reply(w, 201, map[string]any{"id": id})
}
