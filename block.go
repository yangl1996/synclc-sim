package main

import (
	"time"
)

type BlockMetadata struct {
	Timestamp time.Time	// time when the block is mined
	ProcCost time.Duration	// processing time of the block
	Hash int
	Round int
	Size int
	Height int
	Parent int
	Invalid bool	// causes the block to be processed but then dropped
}

type Block struct {
	BlockMetadata
	Data []byte
}

func (b *BlockMetadata) process() {
	time.Sleep(b.ProcCost)
}
