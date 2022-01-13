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
	randSeed := flag.Int64("seed", 0, "random number generator seed for miner")
	startUnix := flag.Int64("start", 0, "unix timestamp for start time, 0 for now")
	mineSec := flag.Duration("ri", time.Duration(1) * time.Second, "round interval")
	blockSize := flag.Int("size", 100000, "block size in bytes")
	blockTime := flag.Duration("proc", time.Duration(0) * time.Second, "block processing time")
	procParallel := flag.Int("parallel", 1, "number of workers to start")
	slotMineProb := flag.Float64("lottery", 0.3, "prob to mine a block in a round")
	attacker := flag.Bool("attack", false, "attacker mode")
	outputPath := flag.String("output", "", "prefix of output path")
	downloadRule := flag.String("rule", "longest", "download prioritization rule (longest, freshest)")

	flag.Parse()

	rand.Seed(time.Now().UnixNano())	// this is for hashes so seeds do not matter

	startTime := time.Unix(*startUnix, 0)
	var peers []string
	for _, peerAddr := range strings.Split(*peerList, ",") {
		peers = append(peers, peerAddr)
	}

	var rng *rand.Rand
	if *randSeed == 0 {
		src := rand.NewSource(time.Now().UnixNano())
		rng = rand.New(src)
	} else {
		src := rand.NewSource(*randSeed)
		rng = rand.New(src)
	}
	var dlRule int
	switch *downloadRule {
	case "freshest":
		dlRule = FreshestChainFirst
	case "longest":
		dlRule = LongestChainFirst
	default:
		log.Fatalln("unknown download rule", *downloadRule)
	}
	m := &Miner{rng, *slotMineProb, *mineSec, 1, make(chan int, 100)}
	s, _ := NewServer(fmt.Sprintf("0.0.0.0:%v", *listenPort), *procParallel, *maxInflight, *globalInflight, m, *blockSize, *blockTime, *attacker, dlRule, *outputPath)
	log.Printf("dummy node started, connecting to outgoing peers %v\n", peers)

	if *peerList != "" {
		log.Println("connecting to outgoing peers")
		outgoingWg := &sync.WaitGroup{}
		outgoingWg.Add(len(peers))
		for _, p := range peers {
			go func(p string) {
				defer outgoingWg.Done()
				s.connect(p)
			}(p)
		}
		outgoingWg.Wait()
		log.Println("all outgoing peers connected")
	}

	if *startUnix != 0 {
		time.Sleep(time.Until(startTime))
	}
	m.Mine()
}

