package main

import (
	"net"
	"encoding/gob"
	"log"
)

type Message interface{}

type peerConn struct {
	net.Conn
	*gob.Encoder
	*gob.Decoder
}

func (p *peerConn) sendData(from chan Message) error {
	for m := range from {
		err := p.Encode(&m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *peerConn) receiveData(to chan peerMessage, idx int) error {
	for {
		var msg Message
		err := p.Decode(&msg)
		if err != nil {
			return err
		}
		to <- peerMessage{msg, idx}
	}
}

type peer struct {
	index int
	hpConn *peerConn
	lpConn *peerConn
	inCh chan peerMessage
	hpCh chan Message
	lpCh chan Message
}

func (p *peer) handle() {
	go func() {
		err := p.hpConn.sendData(p.hpCh)
		if err != nil {
			log.Fatalln(err)
		}
	}()
	go func() {
		err := p.lpConn.sendData(p.lpCh)
		if err != nil {
			log.Fatalln(err)
		}
	}()
	go func() {
		err := p.hpConn.receiveData(p.inCh, p.index)
		if err != nil {
			log.Fatalln(err)
		}
	}()
	go func() {
		err := p.lpConn.receiveData(p.inCh, p.index)
		if err != nil {
			log.Fatalln(err)
		}
	}()
	select{}
}

