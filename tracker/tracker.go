package tracker

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/zeebo/bencode"
)

type trackerResponse struct {
	Failure        string `bencode:"failure reason"`
	Warning        string `bencode:"warning message"`
	TrackerID      string `bencode:"tracker id"`
	IntervalSec    int    `bencode:"interval"`
	MinIntervalSec int    `bencode:"min interval"`
	Complete       int    `bencode:"complete"`
	Incomplete     int    `bencode:"incomplete"`
	Peers          []byte `bencode:"peers"`
}

type Tracker struct {
	status     string
	trackerID  string
	interval   int
	latestResp time.Time
}

func New() *Tracker {
	return &Tracker{
		status: "started",
	}
}

func (t *Tracker) GetIntervalSec() int {
	return t.interval
}

func (t *Tracker) doQuerry(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (t *Tracker) AnsQuerry(url string) ([]net.TCPAddr, error) {
	resp, err := t.doQuerry(url)
	if err != nil {
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsedResp trackerResponse
	err = bencode.DecodeBytes(respBody, &parsedResp)
	if err != nil {
		return nil, err
	}

	if parsedResp.Failure != "" || parsedResp.Warning != "" {
		return nil, errors.New("failure reason: " + parsedResp.Failure + " warning reason: " + parsedResp.Warning)
	}

	if parsedResp.TrackerID != "" {
		t.trackerID = parsedResp.TrackerID
	}
	t.interval = max(parsedResp.MinIntervalSec, parsedResp.IntervalSec)
	t.latestResp = time.Now()

	parsedPeers := make([]net.TCPAddr, len(parsedResp.Peers)/6)

	for j, i := 0, 0; i < len(parsedResp.Peers); i += 6 {
		parsedPeers[j] = net.TCPAddr{
			IP: net.IPv4(
				parsedResp.Peers[i],
				parsedResp.Peers[i+1],
				parsedResp.Peers[i+2],
				parsedResp.Peers[i+3]),
			Port: int(binary.BigEndian.Uint16(parsedResp.Peers[i+4 : i+6])),
		}
		j++
	}
	return parsedPeers, nil
}

func (t *Tracker) NoAnsQuerry(url string) error {
	_, err := t.doQuerry(url)
	if err != nil {
		return err
	}
	return nil
}
