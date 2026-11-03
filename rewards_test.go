package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	emit "github.com/openpak/rewards-emit"
)

func TestOnlyCompletedTradesAreReported(t *testing.T) {
	var mu sync.Mutex
	var got []map[string]any
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var in struct{ Events []map[string]any }
		json.Unmarshal(b, &in)
		mu.Lock()
		got = append(got, in.Events...)
		mu.Unlock()
		w.Write([]byte(`{"results":[]}`))
	}))
	defer sink.Close()
	st := testStore(t)
	rw := emit.New(emit.Config{URL: sink.URL, Key: "k", Source: "pgl", Flush: 10 * time.Millisecond})
	h := (&API{Store: st, Check: fakeCheck{}, Key: "k", Rewards: rw}).Routes()

	do(h, "POST", "/internal/v1/gts/deposit", map[string]any{"pid": 1, "game": "x", "format": "pk6", "data": []byte{25, 30, 0},
		"request": map[string]any{"species": 150, "gender": -1, "level_max": 100}}, "k")
	do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 2, "format": "pk6", "data": []byte{1, 5, 0}}, "k")   // wrong species: refused
	do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 1, "format": "pk6", "data": []byte{150, 5, 0}}, "k") // own listing: refused
	do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 2, "format": "pk6", "data": []byte{150, 5, 0}}, "k") // the trade
	do(h, "POST", "/internal/v1/gts/1/trade", map[string]any{"pid": 3, "format": "pk6", "data": []byte{150, 5, 0}}, "k") // lost race
	do(h, "POST", "/internal/v1/gts/1/finish", map[string]any{"pid": 1}, "k")                                            // owner collects
	rw.Close(context.Background())

	mu.Lock()
	defer mu.Unlock()
	ids := map[string]string{}
	for _, e := range got {
		ids[e["event_id"].(string)] = e["identity"].(map[string]any)["subject"].(string)
		if e["identity"].(map[string]any)["namespace"] != "3ds" {
			t.Fatalf("gen 6 is a 3DS trainer: %v", e)
		}
	}
	if len(got) != 2 || ids["gts:1:taker"] != "2" || ids["gts:1:owner"] != "1" {
		t.Fatalf("events %v", got)
	}
}
