package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

type Peer struct {
	conn         net.Conn
	choking      bool
	interested   bool
	amChoking    bool
	amInterested bool
	infoHash     [20]byte
}

func NewPeer(conn net.Conn, infohash [20]byte) *Peer {
	return &Peer{
		conn:      conn,
		choking:   true,
		amChoking: true,
		infoHash:  infohash,
	}
}

type Server struct {
	connLimit int
	currConn  int
}

func NewServer(connLimit int) *Server {
	return &Server{
		connLimit: connLimit,
	}
}

func (s *Server) loopHandler(conn net.Conn) error {
	for {

	}
}

func (s *Server) HandleConn(conn net.Conn) (err error) {
	defer func() {
		if err != nil {
			fmt.Println(err.Error())
		}
		err = conn.Close()
		if err != nil {
			fmt.Println(err.Error())
		}
		s.currConn--
	}()
	s.currConn++

	if s.currConn > s.connLimit {
		return errors.New("connection limit exceeded")
	}

	return s.loopHandler(conn)
}

func (s *Server) Listen(port int) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		go s.HandleConn(conn)
	}
}
