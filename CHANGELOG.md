# Changelog — pokemon-global-link

## v0.3.0 — 2026-11-04

- Account erasure (card #120): `DELETE /internal/accounts/{account_id}/data?pid=<trainer id>` with `Authorization: Bearer $ACCOUNT_ERASE_KEY` on its own listener `ERASE_ADDR` (default `:9099`, container network only, not the public API port). Unset key = 503, bad key = 401, non-UUID = 400. Called by pokemon-home, which owns the account to trainer-id mapping.
- Erased for the named trainer id(s): GTS listings (open, traded and uncollected) and battle records with their members and moves. Nothing else in this store can be tied to a person: it has no account id. Without `pid` the call changes nothing (0 rows). Idempotent.
- Kept: nothing for that trainer. Showcase usage statistics are recomputed from what remains.

## v0.2.0 — 2026-11-03

- State moved from SQLite (`pgl.db`) to Postgres (`PGL_DATABASE_URL`, required; no file fallback, the service refuses to start without it). Small pool of 3. The SQLite dependency is gone.
- rewards-emit v0.3.0: set `REWARDS_EMIT_DATABASE_URL` for a durable rewards outbox.
- Restart test; Postgres tests run when `PGL_TEST_DATABASE_URL` is set.

## v0.1.1 — 2026-11-01

- Completed GTS trades (taker on accept, owner on collect) are reported to the rewards service (`REWARDS_EVENTS_URL` / `REWARDS_EVENTS_KEY`; unset = off).

## v0.1.0 — 2026-11-01

- GTS store (deposit, search, trade, withdraw/collect), battle records, Global Showcase usage rankings.
- Public read-only JSON API and internal API; legality gate via pokemon-legality on every Pokemon.
