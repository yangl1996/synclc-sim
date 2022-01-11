package main

import (
	"net"
	"log"
	"sync"
	"time"
	"encoding/gob"
	"math/rand"
	"github.com/hashicorp/yamux"
)

type peerMessage struct {
	Message
	from int
}

type peerHandle struct {
	chain []BlockMetadata
	inflight map[int]struct{}
	hpCh chan Message
	lpCh chan Message
	downloadPtr int

	// the following fields are only used by the attacker
	fakeTip BlockMetadata // our tip as believed by the victim
	// tracks the attackable tickets
	// these fields are not used for now; they are intended to track
	// the best valid block as the base for the attack; we want to maximize
	// the achievable height of equivocating chains
	bestAttackFirstTicketIndex int
	bestAttackTargetTip BlockMetadata
}

type Server struct {
	lock *sync.Mutex
	peerMsg chan peerMessage

	peers []*peerHandle
	globalCap int
	localCap int

	blockBuffer map[int]BlockMetadata
	validatedBlocks map[int]BlockMetadata	// downloaded and validated blocks
	inflight map[int]struct{}
	downloaded map[int]struct{}
	adoptedTip BlockMetadata

	processorCh chan BlockMetadata

	miner *Miner
	blockSize int
	blockProcCost time.Duration

	// the following fields are only used by the attacker
	attacker bool
	tickets []int
}

func (s *Server) produceHonestBlocks() {
	for r := range s.miner.tickets {
		s.lock.Lock()
		nb := BlockMetadata {
			Timestamp: time.Now(),
			ProcCost: s.blockProcCost,
			Hash: rand.Int(),
			Round: r,
			Size: s.blockSize,
			Height: s.adoptedTip.Height+1,
			Parent: s.adoptedTip.Hash,
			Invalid: false,
		}
		s.newValidatedBlock(nb)
		s.lock.Unlock()
	}
}

func (s *Server) collectAttackTickets() {
	for r := range s.miner.tickets {
		s.lock.Lock()
		s.tickets = append(s.tickets, r)
		nPeers := len(s.peers)
		s.lock.Unlock()
		for i := 0; i < nPeers; i++ {
			s.tryProduceAttackBlocks(i)
		}
	}
}

func (s *Server) tryProduceAttackBlocks(forPeer int) {
	s.lock.Lock()

	var targetTip BlockMetadata

	// We can attack when we have a longer chain than peer's adopted chain.
	// Search from peer's adopted tip backwards to find the longest chain
	// we can build, given that round numbers should be strictly increasing.

	// collect tickets valid for the attack
	attackTickets := []int{}
	ticketPtr := len(s.tickets)-1	// the next ticket to consider
	tipPtr := len(s.peers[forPeer].chain)-1	// the next tip index to consider
	victimHeight := s.peers[forPeer].chain[tipPtr].Height

	// search for at most 30 blocks back
	searched := 0
	bestHeightDiff := 0
	bestTicketCount := 0
	for tipPtr >= 0 && searched < 30 {
		// collect more tickets valid for this tipPtr
		for ticketPtr >= 0 && s.tickets[ticketPtr] > s.peers[forPeer].chain[tipPtr].Round {
			attackTickets = append(attackTickets, s.tickets[ticketPtr])
			ticketPtr -= 1
		}
		ourHeight := s.peers[forPeer].chain[tipPtr].Height + len(attackTickets)
		if ourHeight - victimHeight > bestHeightDiff {
			bestHeightDiff = ourHeight - victimHeight
			bestTicketCount = len(attackTickets)
			targetTip = s.peers[forPeer].chain[tipPtr]
		}
		tipPtr -= 1
		searched += 1
	}

	s.lock.Unlock()
	if bestHeightDiff == 0 {
		// no ticket for attacking
		return
	}
	s.lock.Lock()
	log.Printf("mining spam chain on top of %v (height %v round %v) to height %v round %v\n", targetTip.Hash, targetTip.Height, targetTip.Round, targetTip.Height+bestTicketCount, attackTickets[0])
	// make sure we have all peer's block in the validated set otherwise we will
	// make mistake when computing chain diff
	ptr := len(s.peers[forPeer].chain)-1
	for ptr >= 0 {
		if _, there := s.validatedBlocks[s.peers[forPeer].chain[ptr].Hash]; there {
			break
		} else {
			s.validatedBlocks[s.peers[forPeer].chain[ptr].Hash] = s.peers[forPeer].chain[ptr]
		}
		ptr -= 1
	}
	for tidx := bestTicketCount-1; tidx >= 0; tidx-- {
		nb := BlockMetadata {
			Timestamp: time.Now(),
			ProcCost: s.blockProcCost,
			Hash: rand.Int(),
			Round: attackTickets[tidx],
			Size: s.blockSize,
			Height: targetTip.Height+1,
			Parent: targetTip.Hash,
			Invalid: true,
		}
		targetTip = nb
		// insert the spam block
		s.validatedBlocks[nb.Hash] = nb
	}
	added, removed := s.computeDiff(s.peers[forPeer].fakeTip, targetTip)
	s.peers[forPeer].fakeTip = targetTip
	// memory optimization: remove the old blocks from the map
	for _, b := range removed {
		if b.Invalid {
			delete(s.validatedBlocks, b.Hash)
		}
	}
	msg := &ChainUpdate {
		added,
		removed,
	}
	s.peers[forPeer].hpCh <- msg
	s.lock.Unlock()
}

func NewServer(addr string, ncores int, localCap int, globalCap int, miner *Miner, blockSize int, blockProcCost time.Duration, attacker bool) (*Server, error) {
	s := &Server {
		lock: &sync.Mutex{},
		peerMsg: make(chan peerMessage, 1000),
		globalCap: globalCap,
		localCap: localCap,
		validatedBlocks: make(map[int]BlockMetadata),
		inflight: make(map[int]struct{}),
		downloaded: make(map[int]struct{}),
		processorCh: make(chan BlockMetadata, 512),
		blockBuffer: make(map[int]BlockMetadata),
		miner: miner,
		blockSize: blockSize,
		blockProcCost: blockProcCost,
		attacker: attacker,
	}
	// genesis block
	s.validatedBlocks[0] = BlockMetadata{}
	s.downloaded[0] = struct{}{}
	go func() {
		err := s.listenForPeers(addr)
		if err != nil {
			log.Fatalln(err)
		}
	}()
	s.processDownloadedBlocks(ncores)
	go s.processMessages()
	if !attacker {
		go s.produceHonestBlocks()
	} else {
		go s.collectAttackTickets()
	}
	return s, nil
}

type BlockRequest struct {
	Header BlockMetadata
}

func (s *Server) processMessages() {
	for pm := range s.peerMsg {
		from := pm.from
		msg := pm.Message
		switch m := msg.(type) {
		case *Block:
			s.lock.Lock()
			delete(s.peers[from].inflight, m.Hash)
			delete(s.inflight, m.Hash)
			s.downloaded[m.Hash] = struct{}{}
			s.lock.Unlock()
			s.processorCh <- m.BlockMetadata
		case *ChainUpdate:
			s.lock.Lock()
			for ridx := 0; ridx < len(m.Removed); ridx++ {
				lastIdx := len(s.peers[from].chain)-1
				if s.peers[from].chain[lastIdx].Hash != m.Removed[ridx].Hash {
					log.Fatalln("rolling back from incorrect tip")
				}
				s.peers[from].chain = s.peers[from].chain[0:lastIdx]
				if s.peers[from].downloadPtr > lastIdx {
					s.peers[from].downloadPtr = lastIdx
				}
			}
			// added blocks are ordered from high to low, so we need to
			// iterate in reverse order
			for aidx := len(m.Added)-1; aidx >= 0; aidx-- {
				lastIdx := len(s.peers[from].chain)-1
				if s.peers[from].chain[lastIdx].Hash != m.Added[aidx].Parent {
					log.Fatalf("peer %v adding block to incorrect tip %v, should be %v\n", from, s.peers[from].chain[lastIdx].Hash, m.Added[aidx].Parent)
				}
				s.peers[from].chain = append(s.peers[from].chain, m.Added[aidx])
			}
			newTip := s.peers[from].chain[len(s.peers[from].chain)-1]
			s.lock.Unlock()
			log.Printf("peer %v chain switch to %v at height %v\n", from, newTip.Hash, newTip.Height)
		case *BlockRequest:
			out := &Block{}
			if !s.attacker {
				s.lock.Lock()
				_, there := s.validatedBlocks[m.Header.Hash]
				if !there {
					log.Fatalf("requested block body %v has not been validated\n", m.Header.Hash)
				}
				out.BlockMetadata = s.validatedBlocks[m.Header.Hash]
				s.lock.Unlock()
			} else {
				out.BlockMetadata = m.Header
			}
			out.Data = make([]byte, out.Size)
			rand.Read(out.Data)
			s.lock.Lock()
			s.peers[from].lpCh <- out
			s.lock.Unlock()
		default:
			panic("unhandled message")
		}
		if !s.attacker {
			s.tryRequestNextBlock()
		} else {
			s.tryProduceAttackBlocks(from)
		}
	}
}

func (s *Server) tryRequestNextBlock() {
	s.lock.Lock()
	defer s.lock.Unlock()

	// request blocks until there is no peer to download from, or we have
	// filled the global cap
	tried := make(map[int]struct{})
	for s.globalCap > len(s.inflight) && len(tried) < len(s.peers) {
		// rule for deciding which peer(s) to fetch from
		bestPeer := -1
		bestHeight := s.adoptedTip.Height
		for pidx := range s.peers {
			// do not try peers that has been tried
			if _, there := tried[pidx]; there {
				continue
			}
			// do not request if we run out of local quota
			if len(s.peers[pidx].inflight) >= s.localCap {
				tried[pidx] = struct{}{}
				continue
			}
			// advance the download ptr
			ptr := s.peers[pidx].downloadPtr
			for ptr < len(s.peers[pidx].chain) {
				_, downloaded := s.downloaded[s.peers[pidx].chain[ptr].Hash]
				_, inflight := s.inflight[s.peers[pidx].chain[ptr].Hash]
				if (!downloaded) && (!inflight) {
					break
				} else {
					ptr += 1
				}
			}
			s.peers[pidx].downloadPtr = ptr
			// do not request if the pointer is already out of scope
			if len(s.peers[pidx].chain) <= s.peers[pidx].downloadPtr {
				tried[pidx] = struct{}{}
				continue
			}
			peerTip := s.peers[pidx].chain[len(s.peers[pidx].chain)-1]
			if peerTip.Height > bestHeight {
				bestPeer = pidx
				bestHeight = peerTip.Height
			}
		}
		if bestPeer == -1 {
			break
		} else {
			toRequest := s.peers[bestPeer].chain[s.peers[bestPeer].downloadPtr]
			s.peers[bestPeer].downloadPtr += 1
			log.Printf("requesting %v\n", toRequest.Hash)
			msg := &BlockRequest{toRequest}
			s.peers[bestPeer].hpCh <- msg
			s.inflight[toRequest.Hash] = struct{}{}
			s.peers[bestPeer].inflight[toRequest.Hash] = struct{}{}
		}
	}
}

func (s *Server) connect(addr string) error {
	backoff := 100	// ms
	var conn net.Conn
	var err error
	for {
		log.Printf("dialing %s\n", addr)
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(time.Duration(backoff) * time.Millisecond)
		} else {
			break
		}
	}
	log.Printf("outgoing connection to %s\n", conn.RemoteAddr().String())
	// outgoing connection, initiate two yamux streams
	session, err := yamux.Client(conn, nil)
	if err != nil {
		return err
	}
	hpStr, err := session.Open()
	if err != nil {
		return err
	}
	lpStr, err := session.Open()
	if err != nil {
		return err
	}
	s.lock.Lock()
	idx := len(s.peers)
	handle := &peerHandle {
		chain: []BlockMetadata{BlockMetadata{}},
		inflight: make(map[int]struct{}),
		hpCh: make(chan Message, 1000),
		lpCh: make(chan Message, 1000),
	}
	s.peers = append(s.peers, handle)
	peer := &peer {
		index: idx,
		hpConn: &peerConn {
			hpStr,
			gob.NewEncoder(hpStr),
			gob.NewDecoder(hpStr),
		},
		lpConn: &peerConn {
			lpStr,
			gob.NewEncoder(lpStr),
			gob.NewDecoder(lpStr),
		},
		inCh: s.peerMsg,
		hpCh: handle.hpCh,
		lpCh: handle.lpCh,
	}
	s.lock.Unlock()
	go peer.handle()
	return nil
}

func (s *Server) listenForPeers(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		log.Printf("incoming connection from %s\n", conn.RemoteAddr().String())
		// incoming connection, wait for two yamux streams
		session, err := yamux.Server(conn, nil)
		if err != nil {
			return err
		}
		hpStr, err := session.Accept()
		if err != nil {
			return err
		}
		lpStr, err := session.Accept()
		if err != nil {
			return err
		}
		s.lock.Lock()
		idx := len(s.peers)
		handle := &peerHandle {
			chain: []BlockMetadata{BlockMetadata{}},
			inflight: make(map[int]struct{}),
			hpCh: make(chan Message, 1000),
			lpCh: make(chan Message, 1000),
		}
		s.peers = append(s.peers, handle)
		peer := &peer {
			index: idx,
			hpConn: &peerConn {
				hpStr,
				gob.NewEncoder(hpStr),
				gob.NewDecoder(hpStr),
			},
			lpConn: &peerConn {
				lpStr,
				gob.NewEncoder(lpStr),
				gob.NewDecoder(lpStr),
			},
			inCh: s.peerMsg,
			hpCh: handle.hpCh,
			lpCh: handle.lpCh,
		}
		s.lock.Unlock()
		go peer.handle()
	}
}

func (s *Server) processDownloadedBlocks(ncores int) {
	serve := func() {
		for block := range s.processorCh {
			s.lock.Lock()
			_, parentDownloaded := s.downloaded[block.Parent]
			_, parentRequested := s.inflight[block.Parent]
			if (!parentDownloaded) && (!parentRequested) {
				log.Fatalf("downloaded block %v whose parent %v has not been downloaded or requested\n", block.Hash, block.Parent)
			}
			parent, parentExists := s.validatedBlocks[block.Parent]
			if !parentExists {
				log.Printf("buffering block %v whose parent %v has not been validated\n", block.Hash, block.Parent)
				s.blockBuffer[block.Parent] = block
				s.lock.Unlock()
				continue
			}
			if parent.Height != block.Height -1 {
				log.Fatalf("block height not incremental (from %v to %v)\n", parent.Height, block.Height)
			}
			if parent.Round >= block.Round {
				log.Fatalf("block round not incremental (from %v to %v)\n", parent.Round, block.Round)
			}
			s.lock.Unlock()

			block.process()
			if block.Invalid {
				continue
			}

			s.lock.Lock()
			s.newValidatedBlock(block)
			if buffered, exists := s.blockBuffer[block.Hash]; exists {
				delete(s.blockBuffer, block.Hash)
				s.processorCh <- buffered
			}
			s.lock.Unlock()
		}
	}
	for i := 0; i < ncores; i++ {
		go serve()
	}
}

func (s *Server) computeDiff(from, to BlockMetadata) (added, removed []BlockMetadata) {
	tip := from
	// figure out the diff
	oldT := tip
	newT := to 
	for oldT.Height != newT.Height {
		if oldT.Height > newT.Height {
			removed = append(removed, oldT)
			oldT = s.validatedBlocks[oldT.Parent]
		} else {
			added = append(added, newT)
			newT = s.validatedBlocks[newT.Parent]
		}
	}
	for oldT.Hash != newT.Hash {
		removed = append(removed, oldT)
		added = append(added, newT)
		if oldT.Height == 1 {
			break
		}
		oldT = s.validatedBlocks[oldT.Parent]
		newT = s.validatedBlocks[newT.Parent]
	}
	return added, removed
}

func (s *Server) newValidatedBlock(block BlockMetadata) {
	log.Printf("processed block %v round %v at time %v\n", block.Hash, block.Round, time.Now().UnixMicro())
	if _, there := s.validatedBlocks[block.Hash]; there {
		// duplicate
		log.Printf("processed duplicate block %v\n", block.Hash)
		return
	}
	s.validatedBlocks[block.Hash] = block
	// compute chain switch
	tip := s.adoptedTip
	if tip.Height < block.Height || (tip.Height == block.Height && tip.Hash > block.Hash) {
		added, removed := s.computeDiff(tip, block)
		s.adoptedTip = block
		if len(removed) != 0 || len(added) != 0 {
			log.Printf("tip switched to block %v height %v at time %v rolling back %v forward %v \n", block.Hash, block.Height, time.Now().UnixMicro(), len(removed), len(added))
			for pidx := range s.peers {
				msg := &ChainUpdate {
					added,
					removed,
				}
				s.peers[pidx].hpCh <- msg
			}
		}
	}
}

type ChainUpdate struct {
	Added []BlockMetadata      // height high -> low
	Removed []BlockMetadata    // height high -> low
}
