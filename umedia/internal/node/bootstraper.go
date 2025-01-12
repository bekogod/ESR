package node

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/Pdf0/umedia/internal/protocol"
	"github.com/Pdf0/umedia/internal/util"
)

type Node struct {
    Ip         []string   `json:"Ip"`
    Neighbours []string `json:"Neighbours"`
}

// StartBootstraper reads the JSON file and returns a slice of Node
func StartBootstraper(filename string) []Node {
    // Open the JSON file
    file, err := os.Open(filename)
    if err != nil {
        log.Fatalf("Failed to open file: %s", err)
    }
    defer file.Close()

    // Read the file content
    byteValue, err := io.ReadAll(file)
    if err != nil {
        log.Fatalf("Failed to read file: %s", err)
    }

    var neighbours []Node

    if err := json.Unmarshal(byteValue, &neighbours); err != nil {
        log.Fatalf("Failed to unmarshal JSON: %s", err)
    }

    return neighbours
}

func GetNeighbours(ip string, list []Node) []string {
	for _, n := range list {
		for i := 0; i < len(n.Ip); i++ {
			if n.Ip[i] == ip {
				return n.Neighbours
			}
		}
	}
	return nil
}

func AskBootstraper(bootstraperIp string) []string {

    log.Println("Getting neighbours from bootstraper...")
    // E quando o bootstraper se conecta a ele próprio?
    conn, err := net.Dial("tcp", bootstraperIp + ":9090")
    if err != nil {
        log.Fatalf("Error while trying to connect to bootstraper %s: %v\n", bootstraperIp, err)
    }
    defer conn.Close()


    p := protocol.NewPacket(2, nil)

    conn.Write(util.EncodeToBytes(p))

    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    if err != nil {
        log.Fatalf("Error while reading from bootstraper %s: %v\n", bootstraperIp, err)
    }

    response := protocol.NewEmptyPacket()
    util.DecodeToStruct(buffer[:n], &response)

    if response.Type == 7 {
        errorPacket := new(protocol.Error)
        util.DecodeToStruct(response.Payload, &errorPacket)
        log.Fatalf("Received and error from bootstraper: %s\n", errorPacket.Message)
    } else if response.Type != 3 {
        log.Fatalf("Unexpected response from bootstraper %s: %s\n", bootstraperIp, protocol.GetType(response.Type))
    }

    neighboursPacket := new(protocol.Neighbours)
    util.DecodeToStruct(response.Payload, &neighboursPacket)

    neighbours := neighboursPacket.Neighbours

    return neighbours
}

func SendNeighbours(neighbours *[]Node, conn net.Conn) {
    toSend := GetNeighbours(strings.Split(conn.RemoteAddr().String(), ":")[0], *neighbours)
    
    var response protocol.Packet
    if toSend == nil {
        errorPacket := protocol.Error{ Message: "IP not on the overlay network!" }

        response = protocol.NewPacket(7, util.EncodeToBytes(errorPacket))
        log.Printf("Sent error to %s\n", conn.RemoteAddr().String())
    } else {
        log.Printf("Sent neighbours to %s", conn.RemoteAddr().String())

        payload := protocol.Neighbours{ Size: uint(len(toSend)), Neighbours: toSend }

        response = protocol.NewPacket(3, util.EncodeToBytes(payload))
    }

    conn.Write(util.EncodeToBytes(response))
    
}
