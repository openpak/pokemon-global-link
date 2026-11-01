# Protocol: pokemon-global-link

This service is the store behind the games' own protocols; the game-facing side is not written yet.
See `research/pokemon/global-link-plan.md` (private research repo) for the per-game analysis.

| Game family | Transport | Status here |
| --- | --- | --- |
| Gen 4 (DP, Pt, HGSS) GTS | GameSpy-era HTTP on `gamestats2.gs.nintendowifi.net`, `/pokemondpds/...` | Not implemented; endpoints recalled from public emulators, no capture in hand |
| Gen 5 (BW, B2W2) GTS | Same host, `/syachi2ds/...`; Global Link via `pokemon-gl.com` | Not implemented; needs capture |
| Gen 6 (XY, ORAS) GTS / Wonder Trade | Game-owned HTTP/NEX service, legality via `pokemon-legality` `/validator/v1/*` | Not implemented; transport unknown, needs a 3DS capture |
| Gen 6 Battle Spot / Battle Video | NEX matchmaking (served) plus a results channel | Not implemented; needs capture |
| Switch SV/PLA | NPLN tenant; Pokemon HOME is a separate Nintendo Account cloud service | Out of scope |

The HTTP API in [api.md](api.md) is complete and tested; the protocol handlers are thin callers of it.
