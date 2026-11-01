# JSON API for the website

All public endpoints are `GET`, return JSON, send `Access-Control-Allow-Origin: *` and
`Cache-Control: public, max-age=15`. Base URL inside the network: `http://pokemon-global-link:20160`.
The website should proxy under `/api/v1/pokemon/...` (strip the prefix) rather than expose the service.
Ids (species, moves, abilities, items, natures) are PKHeX ids; the page maps them to names and art.
Gender: 0 male, 1 female, 2 genderless. Nothing here carries trainer ids or Pokemon files.

## `GET /api/v1/stats`

`{"gts_open": 3, "gts_traded": 12, "battles": 540}`

## `GET /api/v1/gts/listings`

Open listings, newest first. Query: `generation`, `species`, `gender`, `level_min`, `level_max`,
`limit` (1-200, default 50), `offset`.

```json
{"total": 1, "limit": 50, "offset": 0, "listings": [
  {"id": 7, "created_at": 1793550000, "generation": 6, "game": "pokemon-gen6", "species": 25, "form": 0,
   "level": 30, "gender": 0, "shiny": false, "nickname": "PIKACHU", "ot": "RED",
   "request": {"species": 150, "gender": -1, "level_min": 0, "level_max": 100}, "status": "open"}]}
```

`request.species` 0 and `request.gender` -1 mean "any".

## `GET /api/v1/gts/listings/{id}`

One listing (same shape); 404 once withdrawn or collected. A traded listing shows `"status": "traded"`.

## `GET /api/v1/battle/formats`

```json
{"formats": [{"generation": 6, "format": "single", "seasons": ["2026-11", "2026-10"], "battles": 540}]}
```

## `GET /api/v1/showcase`

Global Showcase: usage rankings. Query: `generation`, `format`, `season` (all empty = everything), `limit`
(1-200, default 30).

```json
{"format": "single", "season": "2026-11", "battles": 3,
 "pokemon":   [{"id": 25, "species": 25, "uses": 2, "usage_pct": 0.67, "wins": 1, "losses": 1, "win_rate": 0.5}],
 "moves":     [{"id": 33, "uses": 3, "usage_pct": 1}],
 "abilities": [{"id": 65, "uses": 3, "usage_pct": 1}],
 "items":     []}
```

`usage_pct` is uses divided by `battles`. Rankings are ordered by uses, then id.

## Internal (game-facing handlers only; header `X-Internal-Key`)

`pokemon` objects are `{"format": "pk6", "data": "<base64>"}`. Every Pokemon goes through
`pokemon-legality`; illegal gives 422 with the verdict, an unreachable checker gives 503, and nothing is stored in
either case.

| Call | Body / query | Result |
| --- | --- | --- |
| `POST /internal/v1/gts/deposit` | `{pid, game, request{species,gender,level_min,level_max}, format, data}` | 201 listing; 409 when the trainer already has one in that generation |
| `GET /internal/v1/gts/search` | same filters as public, plus `exclude_pid` | listings with `data` |
| `GET /internal/v1/gts/mine?pid=&generation=` | | own listing incl. `returned` once traded; 404 none |
| `POST /internal/v1/gts/{id}/trade` | `{pid, format, data}` offered Pokemon | 200 the listing (with `data`) if the offer matches the request and the listing was still open; 409 lost race or own listing; 422 mismatch |
| `POST /internal/v1/gts/{id}/finish` | `{pid}` | withdraw an open listing or collect a traded one |
| `POST /internal/v1/battle/record` | `{pid, generation, game, format, season, result: win|loss|draw, team: [pokemon x1-6]}` | 201 `{id}`; whole record refused if any member is illegal |
