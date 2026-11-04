# Deploy: pokemon-global-link

Quadlet `pokemon-global-link.container` on `openpak.network`, image `ghcr.io/openpak/pokemon-global-link:latest`
(`AutoUpdate=registry`), state in Postgres. Port block 20160. Env: `PGL_DATABASE_URL` (required, the service refuses to start without it), `REWARDS_EMIT_DATABASE_URL` (durable rewards outbox), `PGL_INTERNAL_KEY`,
`PGL_LEGALITY_URL=http://pokemon-legality:20170`, `PGL_LEGALITY_KEY` (same as the legality service's
`PKHAX_INTERNAL_KEY`). New quadlets need `pin-ips.sh --apply`. Tags `v*.*.*` build the image.

## Account erasure (card #120)

Global-link stores no account id, only the trainer id (`pid`) a caller deposits under; pokemon-home's pids come from its account table. Erasure is therefore driven by pokemon-home (`DELETE /internal/accounts/{account_id}/data` on pokemon-home), which calls this service's erase listener: `DELETE http://openpak-pokemon-global-link:9099/internal/accounts/{account_id}/data?pid=<pid>` with `Authorization: Bearer <ACCOUNT_ERASE_KEY>`. The listener is `ERASE_ADDR` (default `:9099`), never published to the host; the key unset means 503. Deletes the trainer's GTS listings and battle records (members, moves). Replies `{"service":"pokemon-global-link","rows":N}`. The website does not need its own step for this service.
