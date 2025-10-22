package handshake

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"
)

func validateHandshake(msg, hs []byte) bool {
	if !slices.Equal(msg[:20], hs[:20]) || !slices.Equal(msg[28:48], hs[28:48]) {
		return false
	}
	return true
}

func dialPeer(peer net.TCPAddr) (net.Conn, error) {
	fmt.Printf("Connecting to peer: %s\n", peer.String())
	conn, err := net.DialTimeout(peer.Network(), peer.String(), time.Second*3)
	if err == nil {
		return conn, nil
	}

	return nil, err
}

func readRequestAnswer(conn net.Conn, ansSize int) ([]byte, error) {
	ans := make([]byte, ansSize)
	conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	_, err := conn.Read(ans)

	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func doPeerRequest(conn net.Conn, msg []byte) error {
	_, err := conn.Write(msg)
	if err != nil {
		return err
	}

	return nil
}

func Handshake(peer net.TCPAddr, peerID, infoHash []byte) (net.Conn, error) {
	conn, err := dialPeer(peer)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteByte(byte(19))
	buf.Write([]byte("BitTorrent protocol"))
	buf.Write(make([]byte, 8))
	buf.Write(infoHash)
	buf.Write(peerID)

	err = doPeerRequest(conn, buf.Bytes())
	if err != nil {
		return nil, err
	}

	ans, err := readRequestAnswer(conn, 68)
	if err != nil {
		return nil, err
	}

	valid := validateHandshake(buf.Bytes(), ans)
	if !valid {
		conn.Close()
		return nil, errors.New("invalid peer response")
	}
	return conn, nil
}

func HandshakeAll(peers []net.TCPAddr, peerID, infoHash []byte) []net.Conn {
	res := make([]net.Conn, 0)
	mx := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, p := range peers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := Handshake(p, peerID, infoHash)
			if err != nil {
				fmt.Printf("Failed to handshake %s: %s\n", p.String(), err.Error())
				return
			}
			mx.Lock()
			defer mx.Unlock()
			res = append(res, conn)
		}()
	}
	wg.Wait()
	println(len(res))
	return res
}
