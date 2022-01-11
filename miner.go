package main

import (
	"time"
	"math/rand"
)

type Miner struct {
	rng *rand.Rand
	succProb float64	// probability to win a ticket in a round
	intv time.Duration
	currRound int
	tickets chan int
}

func (m *Miner) Mine() {
	if m.currRound <= 0 {
		panic("invalid starting round number")
	}
	ticker := time.NewTicker(m.intv)
	for {
		select {
		case <- ticker.C:
			if m.rng.Float64() < m.succProb {
				m.tickets <- m.currRound
			}
			m.currRound += 1
		}
	}
}
