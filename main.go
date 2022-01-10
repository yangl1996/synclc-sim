package main

import (
	"flag"
	"strings"
	"log"
	"fmt"
	"math/rand"
	"encoding/gob"
	"sync"
	"time"
)

func main() {
	gob.Register(&Block{})
	gob.Register(&BlockRequest{})
	gob.Register(&ChainUpdate{})


	globalInflight := flag.Int("global", 1, "global inflight cap")
	maxInflight := flag.Int("local", 1, "local inflight cap")
	peerList := flag.String("peers", "", "list of outgoing peer addresses separated by commas")
	listenPort := flag.Int("port", 8000, "port to listen to")
	randSeed := flag.Int64("seed", 0, "random number generator seed")
	startUnix := flag.Int64("start", 0, "unix timestamp for start time, 0 for now")
	mineSec := flag.Duration("ri", time.Duration(1) * time.Second, "round interval")
	blockSize := flag.Int("size", 1000000, "block size in bytes")
	blockTime := flag.Duration("proc", time.Duration(0) * time.Second, "block processing time")
	procParallel := flag.Int("parallel", 1, "number of workers to start")
	slotMineProb := flag.Float64("lottery", 0.3, "prob to mine a block in a round")
	attacker := flag.Bool("attack", false, "attacker mode")

	flag.Parse()

	rand.Seed(*randSeed)

	startTime := time.Unix(*startUnix, 0)
	var peers []string
	for _, peerAddr := range strings.Split(*peerList, ",") {
		peers = append(peers, peerAddr)
	}

	m := &Miner{*slotMineProb, *mineSec, 1, make(chan int, 100)}
	s, _ := NewServer(fmt.Sprintf("0.0.0.0:%v", *listenPort), *procParallel, *maxInflight, *globalInflight, m, *blockSize, *blockTime, *attacker)
	log.Printf("dummy node started, connecting to outgoing peers %v\n", peers)

	if *peerList != "" {
		outgoingWg := &sync.WaitGroup{}
		outgoingWg.Add(len(peers))
		for _, p := range peers {
			go func(p string) {
				defer outgoingWg.Done()
				s.connect(p)
			}(p)
		}
		outgoingWg.Wait()
	}
	log.Println("all outgoing peers connected")

	if *startUnix != 0 {
		for time.Now().Before(startTime) {
		}
	}
	m.Mine()
}

