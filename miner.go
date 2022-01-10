package main

import (
	"math/rand"
	"time"
)

type Miner struct {
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
			if rand.Float64() < m.succProb {
				m.tickets <- m.currRound
			}
			m.currRound += 1
		}
	}
}
