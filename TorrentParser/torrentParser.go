package torrentParser

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"

	"github.com/zeebo/bencode"
)

type bencodeFile struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

type bencodeInfo struct {
	Length      int           `bencode:"length"`
	Name        string        `bencode:"name"`
	Files       []bencodeFile `bencode:"files,omitempty"`
	PieceLength int           `bencode:"piece length"`
	Pieces      []byte        `bencode:"pieces"`
}

type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceLength int
	PiecesHash  [][20]byte
	Length      int
	Name        string
}

func (tf *TorrentFile) getBaseURL(port int, peerID [20]byte) string {
	query := url.Values{
		"info_hash": []string{string(tf.InfoHash[:])},
		"port":      []string{strconv.Itoa(port)},
		"compact":   []string{strconv.Itoa(1)},
		"peer_id":   []string{string(peerID[:])},
		"ip":        []string{getOutboundIP().String()},
	}

	return fmt.Sprintf("%s?%s", tf.Announce, query.Encode())
}

func (tf *TorrentFile) constructURL(
	event string,
	uploaded,
	downloaded,
	left,
	port int,
	peerID [20]byte,
) string {
	baseURL := tf.getBaseURL(port, peerID)
	query := url.Values{
		"event":      []string{event},
		"uploaded":   []string{strconv.Itoa(uploaded)},
		"downloaded": []string{strconv.Itoa(downloaded)},
		"left":       []string{strconv.Itoa(left)},
	}

	return fmt.Sprintf("%s&%s", baseURL, query.Encode())
}

func (tf *TorrentFile) GetStartedURL(port int, peerID [20]byte) string {
	return tf.constructURL(
		"started",
		0,
		0,
		tf.Length,
		port,
		peerID,
	)
}

func (tf *TorrentFile) GetContinueURL(port int, peerID [20]byte, uploaded, downloaded int) string {
	return tf.constructURL(
		"",
		uploaded,
		downloaded,
		tf.Length-downloaded,
		port,
		peerID,
	)
}

func (tf *TorrentFile) GetStopURL(port int, peerID [20]byte, uploaded, downloaded int) string {
	return tf.constructURL(
		"stopped",
		uploaded,
		downloaded,
		tf.Length-downloaded,
		port,
		peerID,
	)
}

func (tf *TorrentFile) GetFinishURL(port int, peerID [20]byte, uploaded int) string {
	return tf.constructURL(
		"completed",
		uploaded,
		tf.Length,
		0,
		port,
		peerID,
	)
}

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func (i *bencodeInfo) getPiecesHash() ([][20]byte, error) {
	const splitSize = 20
	resSize := i.Length / i.PieceLength
	if i.Length%i.PieceLength != 0 {
		resSize++
	}
	res := make([][20]byte, resSize)
	if len(i.Pieces)%splitSize != 0 {
		return nil, errors.New("pieces data corrupted")
	}
	for it := 0; it < resSize; it++ {
		copy(res[it][:], i.Pieces[it*splitSize:splitSize*(it+1)])
	}
	return res, nil
}

func (i *bencodeInfo) hash() ([20]byte, error) {
	bytes, err := bencode.EncodeBytes(*i)
	if err != nil {
		return [20]byte{}, err
	}
	return sha1.Sum(bytes), nil

}

type bencodeTorrent struct {
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list,omitempty"`
	Comment      string      `bencode:"comment,omitempty"`
	CreatedBy    string      `bencode:"created by,omitempty"`
	CreationDate int         `bencode:"creation date,omitempty"`
	Encoding     string      `bencode:"encoding,omitempty"`
	Info         bencodeInfo `bencode:"info"`
}

func (bt *bencodeTorrent) toTorrentFile() (*TorrentFile, error) {
	piecesHash, err := bt.Info.getPiecesHash()
	if err != nil {
		return nil, err
	}

	infoHash, err := bt.Info.hash()
	if err != nil {
		return nil, err
	}

	return &TorrentFile{
		Announce:    bt.Announce,
		Name:        bt.Info.Name,
		Length:      bt.Info.Length,
		PieceLength: bt.Info.PieceLength,
		PiecesHash:  piecesHash,
		InfoHash:    infoHash,
	}, nil
}

func Process(r io.Reader) (*TorrentFile, error) {
	var decoded bencodeTorrent
	decoder := bencode.NewDecoder(r)
	err := decoder.Decode(&decoded)
	if err != nil {
		return nil, err
	}
	res, err := decoded.toTorrentFile()
	if err != nil {
		return nil, err
	}
	return res, nil
}
