# Deploy: pokemon-global-link

Quadlet `pokemon-global-link.container` on `openpak.network`, image `ghcr.io/openpak/pokemon-global-link:latest`
(`AutoUpdate=registry`), volume on `/data` (SQLite `pgl.db`). Port block 20160. Env: `PGL_INTERNAL_KEY`,
`PGL_LEGALITY_URL=http://pokemon-legality:20170`, `PGL_LEGALITY_KEY` (same as the legality service's
`PKHAX_INTERNAL_KEY`). New quadlets need `pin-ips.sh --apply`. Tags `v*.*.*` build the image.
