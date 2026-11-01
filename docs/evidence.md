# Evidence: pokemon-global-link

- `go test ./...` 2026-11-01: GTS deposit/trade/finish flow, illegal refused, checker-down refused,
  battle record and showcase aggregation, public API leaks no pid or file.
- Legality integration verified against a live `pokemon-legality` (see its evidence.md).
- No console has talked to this service yet.
- Deployed 2026-11-01 (box 145.241.199.19, 10.89.2.222 legality, 10.89.2.223 global-link): an illegal Chespin
  (EVs) deposit gave 422, a legal one 201 and showed in the public listing without pid or file; the test row
  was withdrawn afterwards.
