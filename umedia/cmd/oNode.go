package main

import (
	"flag"
	"log"
	"time"

	"github.com/Pdf0/umedia/internal/node"
	"github.com/Pdf0/umedia/internal/protocol"
)

func main() {

	port := "9090"
	bootstraperIp := flag.String("b", "", "Bootstraper Ip")
	nType := flag.String("t", "", "Type of Node")
	streamsNames := flag.String("s", "", "Videos to stream, separated by comma")
	flag.Parse()
	var neighbours []string
	var clients []node.Request

	if *nType != "s" && *nType != "b" && *nType != "p" && *nType != ""  {
		log.Fatalln("Error in node type.")
	}

	b_neighbours := []node.Node{}
	if *nType == "b" {
		b_neighbours = node.StartBootstraper("bootstraper.json")

		neighbours = node.GetNeighbours(*bootstraperIp, b_neighbours)
		log.Printf("Neighbours loaded: %s\n", neighbours)
	} else {
		neighbours = node.AskBootstraper(*bootstraperIp)
	}
	// Temporary fix since the variable is not yet being used, remove later
	neighbours = neighbours

	if *nType == "s" {
		
		if *streamsNames == "" {
			log.Fatalln("Please provide at least one video to broadcast. Place them in the 'videos' folder.")
		}

		streams := node.GetServerStreams(streamsNames)

		
		go func () {
			for {
				timestamp := uint(time.Now().UnixMicro())
				p := protocol.NewHelloServer(streams, timestamp, 0)
				node.SendHelloServerMulticast(neighbours, p)

				time.Sleep(5 * time.Second)
			}
		}()

		node.FillRequests(streams, &clients)
		go node.StreamVideos(streams, &clients)

	}

	allStreams := []node.StreamPath{}
	go node.StartListeningUDP(port, *nType, &allStreams)
	node.StartListeningTCP(port, *nType, neighbours, b_neighbours, &allStreams, clients)
}

