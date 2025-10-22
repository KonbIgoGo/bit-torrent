package remotePeer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/KonbIgoGo/Bit-torrent/bitfield"
	"github.com/KonbIgoGo/Bit-torrent/message"
)

type RemotePeer struct {
	resChan chan<- message.Message
	dieChan chan<- string

	conn          net.Conn
	noMsgTime     *time.Timer
	estSpeedBytes int
	bitfield      bitfield.Bitfield
	peerMx        sync.RWMutex

	localChoking    bool
	localInterested bool
	interested      bool
	choking         bool
}

func New(
	ctx context.Context,
	pieceAmount int,
	conn net.Conn,
	resChan chan<- message.Message,
	dieChan chan<- string) *RemotePeer {
	peer := &RemotePeer{
		resChan:  resChan,
		dieChan:  dieChan,
		bitfield: make([]byte, pieceAmount/8+pieceAmount%8),

		conn:         conn,
		localChoking: true,
		choking:      true,
		noMsgTime:    time.NewTimer(time.Minute * 2),
	}
	go peer.listenMsg(ctx)
	return peer
}

func (p *RemotePeer) GetNoMsgTimeChan() <-chan time.Time {
	return p.noMsgTime.C
}

func (p *RemotePeer) KillPeer() {
	defer p.noMsgTime.Stop()
	p.dieChan <- p.conn.LocalAddr().String()
	err := p.conn.Close()
	if err != nil {
		fmt.Printf("err closing peer connection: %s", err.Error())
	}
}

func (p *RemotePeer) HasPiece(idx int) bool {
	p.peerMx.RLock()
	defer p.peerMx.RUnlock()
	return p.bitfield.HasPiece(idx)
}

func (p *RemotePeer) IsInterested() bool {
	p.peerMx.RLock()
	defer p.peerMx.RUnlock()
	return p.interested
}

func (p *RemotePeer) IsChoking() bool {
	p.peerMx.RLock()
	defer p.peerMx.RUnlock()
	return p.choking
}

func (p *RemotePeer) IsLocalInterested() bool {
	p.peerMx.RLock()
	defer p.peerMx.RUnlock()
	return p.localInterested
}

func (p *RemotePeer) IsLocalChoking() bool {
	p.peerMx.RLock()
	defer p.peerMx.RUnlock()
	return p.localChoking
}

func (p *RemotePeer) handleMessage(msg message.Message) error {
	p.peerMx.Lock()
	defer p.peerMx.Unlock()

	// fmt.Printf("handling msg\n")
	switch msg.GetMsgType() {
	case message.ChokeMsg:
		fmt.Printf("peer %s choked\n", p.conn.LocalAddr().String())
		p.choking = true
	case message.UnchokeMsg:
		fmt.Printf("peer %s unchoked\n", p.conn.LocalAddr().String())
		p.choking = false
	case message.InterestedMsg:
		fmt.Printf("peer %s interested\n", p.conn.LocalAddr().String())
		p.interested = true
	case message.NotInterestedMsg:
		fmt.Printf("peer %s notinterested\n", p.conn.LocalAddr().String())
		p.interested = false
	case message.HaveMsg:
		fmt.Printf("peer %s have\n", p.conn.LocalAddr().String())
		p.bitfield.SetPiece(int(binary.BigEndian.Uint32(msg.GetPayload()[0])))
	case message.BitfieldMsg:
		bitfield := msg.GetPayload()[0]
		if len(bitfield) != len(p.bitfield) {
			fmt.Printf("error handling bitfield %s\n", p.conn.LocalAddr().String())
			p.KillPeer()
		} else {
			fmt.Printf("peer %s bitfield\n", p.conn.LocalAddr().String())
			p.bitfield = msg.GetPayload()[0]
		}
	case message.PieceMsg:
		// fmt.Printf("peer %s piece\n", p.conn.LocalAddr().String())
		p.resChan <- msg
	case message.KeepAlive:
		fmt.Printf("KeepAlive msg\n")
	default:
		return fmt.Errorf("unsupported message type: %d", msg.GetMsgType())
	}
	p.noMsgTime.Reset(time.Minute * 2)
	return nil
}

func (p *RemotePeer) SendMsg(data []byte) error {
	_, err := p.conn.Write(data)
	if err != nil {
		fmt.Printf("error sending message to peer: %s", err.Error())
		return err
	}
	return nil
}

func (p *RemotePeer) SendInterested() error {
	fmt.Printf("sending interested in %s\n", p.conn.LocalAddr().String())
	err := p.SendMsg(message.MarshallInterestedMessage())
	if err != nil {
		fmt.Printf("error sending interested message to peer: %s", err.Error())
		return err
	}
	p.peerMx.Lock()
	defer p.peerMx.Unlock()

	p.localInterested = true
	return nil
}

func (p *RemotePeer) SendUnInterested() error {
	fmt.Printf("sending uninterested in %s\n", p.conn.LocalAddr().String())
	err := p.SendMsg(message.MarshallInterestedMessage())
	if err != nil {
		fmt.Printf("error sending uninterested message to peer: %s", err.Error())
		return err
	}
	p.peerMx.Lock()
	defer p.peerMx.Unlock()

	p.localInterested = false
	return nil
}

func (p *RemotePeer) SendRequest(index, begin, length uint32) error {
	// fmt.Printf("sending request  %s\n", p.conn.LocalAddr().String())
	err := p.SendMsg(message.MarshallRequestMessage(index, begin, length))
	if err != nil {
		fmt.Printf("error sending request message to peer: %s", err.Error())
		return err
	}
	return nil
}

func (p *RemotePeer) listenMsg(ctx context.Context) error {
	defer p.KillPeer()

	lenData := make([]byte, 4)
	for {
		select {
		case <-ctx.Done():
			return errors.New("context closed")
		default:
			err := binary.Read(p.conn, binary.BigEndian, lenData)
			if err == io.EOF {
				continue
			}

			if err != nil {
				return err
			}

			len := binary.BigEndian.Uint32(lenData)

			data := make([]byte, len)
			err = binary.Read(p.conn, binary.BigEndian, data)

			if err != nil {
				return err
			}

			msg, err := message.ParseMessage(data)

			if err != nil {
				fmt.Printf("error parsing message: %s\n", err.Error())
				continue
			}

			err = p.handleMessage(msg)

			if err != nil {
				fmt.Printf("error handling message: %s\n", err.Error())
				continue
			}
		}
	}

}
