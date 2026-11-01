// Command pokemon-global-link is OpenPak's Pokemon Global Link: the GTS store, battle records and
// the Global Showcase usage rankings, with a legality gate on every Pokemon that enters.
package main

import (
	emit "github.com/openpak/rewards-emit"
	"log"
	"net/http"
	"os"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	key := os.Getenv("PGL_INTERNAL_KEY")
	if key == "" {
		log.Fatal("PGL_INTERNAL_KEY is required")
	}
	st, err := OpenStore(env("PGL_DB_PATH", "/data/pgl.db"))
	if err != nil {
		log.Fatal(err)
	}
	api := &API{Store: st, Key: key, Check: &HTTPChecker{
		URL: env("PGL_LEGALITY_URL", "http://pokemon-legality:20170"), Key: os.Getenv("PGL_LEGALITY_KEY")},
		Rewards: emit.FromEnv("pokemon-global-link")}
	srv := &http.Server{Addr: env("PGL_HTTP_ADDR", ":20160"), Handler: api.Routes(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
	log.Printf("pokemon-global-link listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
