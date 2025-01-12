package protocol

import (
	"github.com/Pdf0/umedia/internal/util"
	"github.com/google/uuid"
)

/*
Type
0 - HelloServer
1 - HelloClient
2 - GetNeighbours
3 - Neighbours
4 - AvailableStreams
5 - StreamReq
6 - StreamFrame
7 - Error
8 - WantStreams
9 - Ping
10- Pong
11 - StopForwarding
*/

type Packet struct {
	Type     uint8 // Como fazer para ser só 3 bits?
	Payload  []byte
}

type Stream struct {
	Id     uuid.UUID
	Name   string
}

type HelloServer struct {
	ID	      uuid.UUID
	Streams   []Stream
	Timestamp uint
	Delta     uint
}

type Neighbours struct {
	Size       uint
	Neighbours []string
}

type AvailableStreams struct {
	Streams []Stream
}

type StreamReq struct {
	StreamID   uuid.UUID
	PortToSend uint
}

type StreamFrame struct {
	StreamId uint
	FrameId  uint
	FrameNum uint
	IsLast   bool
	Payload  []byte
}

type WantStreams struct {
    ClientIP string
}

type Ping struct {
	StreamID  uuid.UUID
}

type Pong struct {
    Delta uint
}

type Error struct {
	Message string
}

type StopForwarding struct {
    StreamID uuid.UUID
}

func NewEmptyPacket() Packet {
	return Packet{}
}

func NewPacket (type_ uint8, payload []byte) Packet {
	return Packet{
		Type: type_,
		Payload: payload,
	}
}

func NewHelloServer(streams []Stream, timestamp uint, delta uint) Packet {
	helloPacket := HelloServer{
		ID: uuid.New(),
		Streams: streams,
		Timestamp: timestamp,
		Delta: delta,
	}
	return NewPacket(0, util.EncodeToBytes(helloPacket))
}

// Function to translate type uint to it's name
func GetType(type_ uint8) string {
	switch type_ {
	case 0:
		return "HelloServer"
	case 1:
		return "HelloClient"
	case 2:
		return "GetNeighbours"
	case 3:
		return "Neighbours"
	case 4:
		return "AvailableStreams"
	case 5:
		return "StreamReq"
	case 6:
		return "StreamFrame"
	case 7:
		return "Error"
	case 8:
		return "WantStreams"
	case 9:
		return "TypePing"
	case 10:
		return "TypePong"
	case 11:
		return "StopForwarding"
	default:
		return "Unknown"
	}
}
