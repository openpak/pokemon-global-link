# Changelog — pokemon-global-link

## v0.1.1 — 2026-11-01

- Completed GTS trades (taker on accept, owner on collect) are reported to the rewards service (`REWARDS_EVENTS_URL` / `REWARDS_EVENTS_KEY`; unset = off).

## v0.1.0 — 2026-11-01

- GTS store (deposit, search, trade, withdraw/collect), battle records, Global Showcase usage rankings.
- Public read-only JSON API and internal API; legality gate via pokemon-legality on every Pokemon.
