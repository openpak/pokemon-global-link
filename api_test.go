package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// fakeCheck: first byte is the species, second the level, third 1 = illegal.
type fakeCheck struct{ down bool }

func (f fakeCheck) Check(_ context.Context, d []byte, _ string) (*Verdict, error) {
	if f.down {
		return nil, context.DeadlineExceeded
	}
	return &Verdict{Valid: d[2] == 0, Parsed: true, Generation: 6, Species: int(d[0]), Level: int(d[1]), Gender: 0,
		Ability: 65, HeldItem: 0, Moves: []int{33, 45}, Issues: nil}, nil
}

func setup(t *testing.T, c Checker) http.Handler {
	st, err := OpenStore(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	return (&API{Store: st, Check: c, Key: "k"}).Routes()
}

func do(h http.Handler, method, path string, body any, key string) (int, map[string]any) {
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	if key != "" {
		r.Header.Set("X-Internal-Key", key)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestGTSFlow(t *testing.T) {
	h := setup(t, fakeCheck{})
	dep := map[string]any{"pid": 1, "game": "x", "format": "pk6", "data": []byte{25, 30, 0},
		"request": map[string]any{"species": 150, "gender": -1, "level_min": 0, "level_max": 100}}
	if c, _ := do(h, "POST", "/internal/v1/gts/deposit", dep, ""); c != 401 {
		t.Fatalf("no key: %d", c)
	}
	if c, m := do(h, "POST", "/internal/v1/gts/deposit", dep, "k"); c != 201 {
		t.Fatalf("deposit %d %v", c, m)
	}
	if c, _ := do(h, "POST", "/internal/v1/gts/deposit", dep, "k"); c != 409 {
		t.Fatalf("second deposit should conflict: %d", c)
	}
	// illegal never stored
	bad := map[string]any{"pid": 2, "game": "x", "format": "pk6", "data": []byte{25, 30, 1}}
	if c, _ := do(h, "POST", "/internal/v1/gts/deposit", bad, "k"); c != 422 {
		t.Fatalf("illegal deposit: %d", c)
	}
	_, pub := do(h, "GET", "/api/v1/gts/listings", nil, "")
	if pub["total"].(float64) != 1 {
		t.Fatalf("public total %v", pub)
	}
	l := pub["listings"].([]any)[0].(map[string]any)
	if _, has := l["data"]; has || l["pid"] != nil {
		t.Fatalf("public listing leaks data or pid: %v", l)
	}
	// wrong species offered
	if c, _ := do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 2, "format": "pk6", "data": []byte{1, 5, 0}}, "k"); c != 422 {
		t.Fatalf("mismatch trade: %d", c)
	}
	// own listing
	if c, _ := do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 1, "format": "pk6", "data": []byte{150, 5, 0}}, "k"); c != 409 {
		t.Fatalf("self trade: %d", c)
	}
	if c, m := do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 2, "format": "pk6", "data": []byte{150, 5, 0}}, "k"); c != 200 || m["status"] != "open" {
		t.Fatalf("trade %d %v", c, m)
	}
	if c, _ := do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 3, "format": "pk6", "data": []byte{150, 5, 0}}, "k"); c != 409 {
		t.Fatalf("double trade: %d", c)
	}
	_, mine := do(h, "GET", "/internal/v1/gts/mine?pid=1&generation=6", nil, "k")
	if mine["status"] != "traded" || mine["returned"] == nil {
		t.Fatalf("mine %v", mine)
	}
	if c, _ := do(h, "POST", "/internal/v1/gts/1/finish", map[string]any{"pid": 1}, "k"); c != 200 {
		t.Fatalf("collect %d", c)
	}
}

func TestLegalityDownRefuses(t *testing.T) {
	h := setup(t, fakeCheck{down: true})
	dep := map[string]any{"pid": 1, "game": "x", "format": "pk6", "data": []byte{25, 30, 0}}
	if c, _ := do(h, "POST", "/internal/v1/gts/deposit", dep, "k"); c != 503 {
		t.Fatalf("want 503 got %d", c)
	}
	_, pub := do(h, "GET", "/api/v1/gts/listings", nil, "")
	if pub["total"].(float64) != 0 {
		t.Fatal("stored while legality down")
	}
}

func TestBattleShowcase(t *testing.T) {
	h := setup(t, fakeCheck{})
	rec := func(result string, sp byte) int {
		c, _ := do(h, "POST", "/internal/v1/battle/record", map[string]any{"pid": 1, "generation": 6, "game": "x", "format": "single", "season": "2026-11",
			"result": result, "team": []map[string]any{{"format": "pk6", "data": []byte{sp, 50, 0}}}}, "k")
		return c
	}
	for _, r := range []struct {
		res string
		sp  byte
	}{{"win", 25}, {"loss", 25}, {"win", 6}} {
		if c := rec(r.res, r.sp); c != 201 {
			t.Fatal(c)
		}
	}
	if c, _ := do(h, "POST", "/internal/v1/battle/record", map[string]any{"pid": 1, "generation": 6, "format": "single", "result": "win",
		"team": []map[string]any{{"format": "pk6", "data": []byte{25, 50, 1}}}}, "k"); c != 422 {
		t.Fatalf("illegal team: %d", c)
	}
	_, s := do(h, "GET", "/api/v1/showcase?generation=6&format=single&season=2026-11", nil, "")
	if s["battles"].(float64) != 3 {
		t.Fatalf("battles %v", s)
	}
	top := s["pokemon"].([]any)[0].(map[string]any)
	if top["species"].(float64) != 25 || top["uses"].(float64) != 2 || top["win_rate"].(float64) != 0.5 {
		t.Fatalf("top %v", top)
	}
	if len(s["moves"].([]any)) != 2 {
		t.Fatalf("moves %v", s["moves"])
	}
	_, f := do(h, "GET", "/api/v1/battle/formats", nil, "")
	if len(f["formats"].([]any)) != 1 {
		t.Fatalf("formats %v", f)
	}
}
