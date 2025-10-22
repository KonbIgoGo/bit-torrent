package message

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type messageType int8

const (
	KeepAlive messageType = iota - 1
	ChokeMsg
	UnchokeMsg
	InterestedMsg
	NotInterestedMsg
	HaveMsg
	BitfieldMsg
	RequestMsg
	PieceMsg
)

type Message interface {
	GetMsgType() messageType
	GetPayload() [][]byte
}

var _ Message = (*noPayloadMsg)(nil)
var _ Message = (*payloadMsg)(nil)

type noPayloadMsg struct {
	msgType messageType
}

func newNoPayloadMsg(id messageType) Message {
	return &noPayloadMsg{
		msgType: id,
	}
}

func (np *noPayloadMsg) GetMsgType() messageType {
	return np.msgType
}

func (np *noPayloadMsg) GetPayload() [][]byte {
	return nil
}

type payloadMsg struct {
	msgType messageType
	payload [][]byte
}

func newPayloadMsg(id messageType, payload [][]byte) Message {
	return &payloadMsg{
		msgType: id,
		payload: payload,
	}
}

func (p *payloadMsg) GetMsgType() messageType {
	return p.msgType
}

func (p *payloadMsg) GetPayload() [][]byte {
	return p.payload
}

func ParseMessage(data []byte) (Message, error) {
	if len(data) == 0 {
		return newNoPayloadMsg(KeepAlive), nil
	}

	var msgType messageType = messageType(data[0])

	switch msgType {
	case ChokeMsg, UnchokeMsg, InterestedMsg, NotInterestedMsg:
		return newNoPayloadMsg(msgType), nil
	case HaveMsg, BitfieldMsg:
		payload := make([][]byte, 1)
		payload[0] = data[1:]
		return newPayloadMsg(msgType, payload), nil

	case RequestMsg, PieceMsg:
		payload := make([][]byte, 3)
		payload[0] = data[1:5]
		payload[1] = data[5:9]
		payload[2] = data[9:]
		return newPayloadMsg(msgType, payload), nil

	default:
		return nil, errors.New(fmt.Sprintf("unsupported message type: %d\n", msgType))
	}
}

func marshallMsg(id messageType, payload []byte) []byte {
	data := make([]byte, 0)

	len := len(payload) + 1
	lenData := make([]byte, 4)
	binary.BigEndian.PutUint32(lenData, uint32(len))

	data = append(data, lenData...)
	data = append(data, byte(id))
	data = append(data, payload...)

	return data
}

func MarshallRequestMessage(index, begin, length uint32) []byte {
	data := make([]byte, 12)

	binary.BigEndian.PutUint32(data, index)
	binary.BigEndian.PutUint32(data[4:8], begin)
	binary.BigEndian.PutUint32(data[8:], length)

	return marshallMsg(RequestMsg, data)
}

func MarshallKeepAliveMessage() []byte {
	var res [4]byte
	return res[:]
}

func MarshallInterestedMessage() []byte {
	data := make([]byte, 5)
	binary.BigEndian.PutUint32(data, 1)
	data[4] = byte(2)

	return data
}

func MarshallUnInterestedMessage() []byte {
	data := make([]byte, 5)
	binary.BigEndian.PutUint32(data, 1)
	data[4] = byte(3)

	return data
}
