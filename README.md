# Pokemon Global Link

OpenPak's Pokemon Global Link: GTS store, battle records and the Global Showcase usage rankings,
with a legality gate (`pokemon-legality`) on every Pokemon that enters.

| | |
| --- | --- |
| Network | Any; Gen 4/5 (WFC), Gen 6 (NEX) and later generations feed the same store |
| Role | GTS listings and trades, battle records, usage rankings, public read-only JSON API |
| Depends on | [`pokemon-legality`](https://github.com/openpak/pokemon-legality); SQLite file on a volume |
| Image | `ghcr.io/openpak/pokemon-global-link` |
| Licence | AGPL-3.0-only |

## What it serves

| Function | What |
| --- | --- |
| Public JSON API (`/api/v1/*`) | GTS listings, stats, battle formats, Global Showcase; no trainer ids, no Pokemon files |
| Internal API (`/internal/v1/*`) | Deposit, search, trade, finish, battle record; for the game-facing protocol handlers; needs `X-Internal-Key` |
| Game-facing GTS / battle / showcase protocols | **Not yet**: each needs a console capture first, see [protocol](docs/protocol.md) |

## Run

```sh
cp example.env .env   # set PGL_INTERNAL_KEY
set -a; . ./.env; set +a
GOWORK=off go run .
GOWORK=off go test ./...
```

## Docs

- [Protocol](docs/protocol.md)
- [API for the website](docs/api.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
