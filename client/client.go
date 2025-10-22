package client

import (
	"context"
	"crypto/rand"
	"io"
	"time"

	torrentParser "github.com/KonbIgoGo/Bit-torrent/TorrentParser"
	"github.com/KonbIgoGo/Bit-torrent/handshake"
	"github.com/KonbIgoGo/Bit-torrent/p2p"
	"github.com/KonbIgoGo/Bit-torrent/tracker"
)

func generatePeerID() [20]byte {
	var peerID [20]byte
	clientPrefix := "-GT0001-"
	copy(peerID[:len(clientPrefix)], clientPrefix)
	rand.Read(peerID[len(clientPrefix):])
	return peerID
}

type Client struct {
	tf      *torrentParser.TorrentFile
	tracker *tracker.Tracker
	p2p     *p2p.PeerToPeer
	port    int
	peerID  [20]byte
}

func New(r io.Reader) (*Client, error) {
	tf, err := torrentParser.Process(r)
	if err != nil {
		return nil, err
	}

	return &Client{
		tf:      tf,
		tracker: tracker.New(),
		port:    6881,
		peerID:  generatePeerID(),
	}, nil
}

func (c *Client) StartDownloadingTo(ctx context.Context, path string) error {
	resp, err := c.tracker.AnsQuerry(c.tf.GetStartedURL(c.port, c.peerID))
	if err != nil {
		return err
	}

	peers := handshake.HandshakeAll(resp, c.peerID[:], c.tf.InfoHash[:])

	piecesAmount := c.tf.Length / c.tf.PieceLength
	if c.tf.Length%c.tf.PieceLength != 0 {
		piecesAmount++
	}
	p2p := p2p.New(ctx, peers, c.peerID, c.tf.PiecesHash)

	go func() {
		interval := c.tracker.GetIntervalSec()
		if interval == 0 {
			interval = 60
		}
		ticker := time.NewTicker(time.Second * time.Duration(interval))

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.tf.GetContinueURL(c.port, c.peerID, p2p.GetUploadedBytes(), p2p.GetDownloadedBytes())
			}
		}
	}()

	err = p2p.StartDownloading(path, c.tf.PieceLength, c.tf.Length)
	if err != nil {
		return err
	}
	return nil
}
