package main

import (
	"fmt"

	emit "github.com/openpak/rewards-emit"
)

// playerFor names a trainer for the rewards service. A PID means different things per
// generation, so the generation picks the account-core namespace: Gen 4/5 are WFC profile ids,
// Gen 6/7 are 3DS NEX PIDs, later games are Switch network service account ids (16 hex).
// Trainers whose link rewards cannot resolve simply earn nothing.
func playerFor(generation int, pid int64) *emit.Identity {
	switch {
	case generation <= 5:
		return &emit.Identity{Namespace: "wfc", Subject: fmt.Sprint(pid)}
	case generation <= 7:
		return &emit.Identity{Namespace: "3ds", Subject: fmt.Sprint(pid)}
	default:
		return &emit.Identity{Namespace: "switch", Subject: fmt.Sprintf("%016x", uint64(pid))}
	}
}

// tradeDone reports one side of a completed GTS trade. The event id is shared with
// pokemon-home, which reports the same trades by account, so a trade seen by both counts once.
func (a *API) tradeDone(listing int64, side string, generation int, pid int64) {
	a.Rewards.Event(emit.Event{ID: fmt.Sprintf("gts:%d:%s", listing, side), Metric: "trades_completed", Player: playerFor(generation, pid)})
}
