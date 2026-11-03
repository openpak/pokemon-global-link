# Changelog — pokemon-global-link

## v0.2.0 — 2026-11-03

- State moved from SQLite (`pgl.db`) to Postgres (`PGL_DATABASE_URL`, required; no file fallback, the service refuses to start without it). Small pool of 3. The SQLite dependency is gone.
- rewards-emit v0.3.0: set `REWARDS_EMIT_DATABASE_URL` for a durable rewards outbox.
- Restart test; Postgres tests run when `PGL_TEST_DATABASE_URL` is set.

## v0.1.1 — 2026-11-01

- Completed GTS trades (taker on accept, owner on collect) are reported to the rewards service (`REWARDS_EVENTS_URL` / `REWARDS_EVENTS_KEY`; unset = off).

## v0.1.0 — 2026-11-01

- GTS store (deposit, search, trade, withdraw/collect), battle records, Global Showcase usage rankings.
- Public read-only JSON API and internal API; legality gate via pokemon-legality on every Pokemon.
