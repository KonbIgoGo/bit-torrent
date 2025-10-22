package p2p

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/KonbIgoGo/Bit-torrent/message"
	"github.com/KonbIgoGo/Bit-torrent/remotePeer"
	"github.com/gammazero/workerpool"
)

type pieceData struct {
	pieceHash         [20]byte
	downloadPieceData [][]byte
	downloadedBlocks  []bool
	pieceID           int
	mx                sync.RWMutex
}

func (p *pieceData) hasBlock(id int) bool {
	p.mx.RLock()
	defer p.mx.RUnlock()
	return p.downloadedBlocks[id]
}

func (p *pieceData) setBlock(id int, data []byte) {
	p.mx.Lock()
	defer p.mx.Unlock()
	p.downloadPieceData[id] = data
	p.downloadedBlocks[id] = true
}

func (p *pieceData) Validate() bool {
	p.mx.Lock()
	defer p.mx.Unlock()
	res := slices.Concat(p.downloadPieceData...)
	hash := sha1.Sum(res)

	return slices.Equal(hash[:], p.pieceHash[:])
}

// type connectionData struct {
// 	downloadedBytes int
// 	uploadedBytes   int
// 	pieceLen        int
// 	blockSizeBytes  int
// 	piecessAmount   int
// }

type PeerToPeer struct {
	peers   map[string]*remotePeer.RemotePeer // stores list of all peers
	peersMx sync.RWMutex

	resChan        chan message.Message
	dieChan        chan string
	blockSizeBytes uint32
	piecesHash     [][20]byte

	bitfield     []byte   // local peer bitfield
	peerID       [20]byte // loacl peer id
	infoHash     [20]byte // SHA1 hash of the torrent file
	currentPiece pieceData

	wp workerpool.WorkerPool
}

func New(ctx context.Context, peers []net.Conn, infoHash [20]byte, piecesHash [][20]byte) *PeerToPeer {
	resChan := make(chan message.Message)
	dieChan := make(chan string)
	res := &PeerToPeer{
		peers:          make(map[string]*remotePeer.RemotePeer),
		piecesHash:     piecesHash,
		infoHash:       infoHash,
		resChan:        resChan,
		dieChan:        dieChan,
		blockSizeBytes: 16384,
		wp:             *workerpool.New(4),
	}
	res.UpdatePeers(ctx, peers)

	go res.keepAlive(ctx, *time.NewTicker(time.Minute))
	go res.listenResChan(ctx)
	go res.listenDieChannel(ctx)
	go res.keepInterested(ctx)

	return res
}

func (p *PeerToPeer) keepInterested(ctx context.Context) {
	// interested in peers that have needed piece
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Printf("keep interested started\n")
			ticker.Stop()
			for _, peer := range p.peers {
				if !peer.IsLocalInterested() && peer.HasPiece(p.currentPiece.pieceID) {
					err := peer.SendInterested()
					if err != nil {
						peer.KillPeer()
					}
				} else if peer.IsLocalInterested() && !peer.HasPiece(p.currentPiece.pieceID) {
					err := peer.SendUnInterested()
					if err != nil {
						peer.KillPeer()
					}
				}
			}
			ticker.Reset(time.Second)
		}
	}
}

func (p *PeerToPeer) listenResChan(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case val, ok := <-p.resChan:
			if !ok {
				return
			}

			payload := val.GetPayload()

			pieceIdx := binary.BigEndian.Uint32(payload[0])
			byteShift := binary.BigEndian.Uint32(payload[1])

			if byteShift%p.blockSizeBytes != 0 {
				fmt.Printf("blockShift is not compatible with block size\n")
				continue
			}

			if pieceIdx != uint32(p.currentPiece.pieceID) {
				fmt.Printf("wrong pieceID while recieveing block of data\n")
				continue
			}

			pieceShift := byteShift / p.blockSizeBytes
			p.currentPiece.setBlock(int(pieceShift), payload[2])
		}
	}
}

func (p *PeerToPeer) listenDieChannel(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case addr, ok := <-p.dieChan:
			fmt.Printf("peer died %s", addr)
			if ok {
				delete(p.peers, addr)
			}
			return
		}
	}
}

func (p *PeerToPeer) GetDownloadedBytes() int {
	panic("not implemented")
}

func (p *PeerToPeer) GetUploadedBytes() int {
	panic("not implemented")
}

func (p *PeerToPeer) UpdatePeers(ctx context.Context, peers []net.Conn) {
	p.peersMx.Lock()
	defer p.peersMx.Unlock()

	for _, peerData := range peers {
		addr := peerData.LocalAddr().String()
		if _, ok := p.peers[addr]; !ok {
			p.peers[addr] = remotePeer.New(ctx, len(p.piecesHash), peerData, p.resChan, p.dieChan)
		}
	}
}

func (p *PeerToPeer) requestBlock(offset, blockLen, pieceID int) {
	for {
		for _, peer := range p.peers {
			if peer.IsLocalInterested() && !peer.IsChoking() && peer.HasPiece(pieceID) {
				// sending request logic
				// fmt.Printf("Sending request msg: %d\n", pieceID)
				err := peer.SendRequest(uint32(pieceID), uint32(offset), uint32(blockLen))
				if err == nil {
					break
				}
			}
		}

		// break conditions
		if p.currentPiece.hasBlock(offset / int(p.blockSizeBytes)) {
			println("downloaded block")
			break
		}
	}
}

func (p *PeerToPeer) downloadPiece(pieceLen, pieceID int) {
	blockAmount := pieceLen / int(p.blockSizeBytes)
	if pieceLen%int(p.blockSizeBytes) != 0 {
		blockAmount++
	}

	p.currentPiece = pieceData{
		pieceHash:         p.piecesHash[pieceID],
		downloadPieceData: make([][]byte, blockAmount),
		downloadedBlocks:  make([]bool, blockAmount),
		pieceID:           pieceID,
	}

	wg := sync.WaitGroup{}
	for {
		fmt.Printf("start downloading piece %d\n", pieceID)
		for i := 0; i < blockAmount; i++ {
			wg.Add(1)
			blockSize := p.blockSizeBytes
			if i == blockAmount-1 {
				blockSize = blockSize - (uint32(pieceLen) % blockSize)
			}
			p.wp.Submit(func() {
				defer wg.Done()
				p.requestBlock(i*int(p.blockSizeBytes), int(blockSize), pieceID)
			})
		}

		wg.Wait()
		if p.currentPiece.Validate() {
			break
		}
	}
	fmt.Printf("downloaded piece %d\n", pieceID)
}

func (p *PeerToPeer) StartDownloading(path string, pieceLen, dataSize int) error {
	piecesIdx := generateRandomPerm(len(p.piecesHash))
	for _, i := range piecesIdx {
		pieceSize := pieceLen
		if i == len(piecesIdx)-1 {
			pieceSize = pieceLen - (dataSize % pieceLen)
		}

		p.downloadPiece(pieceSize, i)
		fmt.Println("Piece downloaded")
	}

	return nil
}

func (p *PeerToPeer) keepAlive(ctx context.Context, ticker time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(p.peers) == 0 {
				continue
			}

			for _, v := range p.peers {
				select {
				case <-v.GetNoMsgTimeChan():
					v.KillPeer()
				default:
					err := v.SendMsg(message.MarshallKeepAliveMessage())
					if err != nil {
						fmt.Printf("error while sending keepAlive message: %s", err.Error())
						v.KillPeer()
					}
				}
			}
		}
	}
}

func generateRandomPerm(len int) []int {
	arr := make([]int, len)
	for i := 0; i < len; i++ {
		arr[i] = i
	}
	rand.Shuffle(len, func(i, j int) {
		arr[i], arr[j] = arr[j], arr[i]
	})
	return arr
}
