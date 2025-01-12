package node

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Pdf0/umedia/internal/protocol"
	"github.com/Pdf0/umedia/internal/util"
	"github.com/google/uuid"
)

type NextHop struct {
	NextHop string
	Delta   uint
	LastHelloID uuid.UUID
}

type StreamPath struct {
	StreamID   	uuid.UUID
	StreamName 	string
	NextHops   	NextHopSlice
}

type NextHopSlice []NextHop

func StartListeningTCP(port string, nType string, neighbours []string, bNeighbours []Node, allStreams *[]StreamPath, clients []Request) {

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Error while trying to listen on port %s: %v\n", port, err)
	}
	defer listener.Close()

	log.Printf("Node listening on port %s...\n", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting the connection:", err)
			continue
		}
		go HandlePacket(conn, nType, &neighbours, &bNeighbours, allStreams, &clients)
	}
}

func HandlePacket(conn net.Conn, nType string, neighbours *[]string, bNeighbours *[]Node, allStreams *[]StreamPath, clients *[]Request) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Error while reading connection from %s:%v", conn.RemoteAddr().String(), err)
		return
	}

	p := protocol.NewEmptyPacket()

	util.DecodeToStruct(buffer[:n], &p)
	log.Printf("Message received from %s: %s\n", conn.RemoteAddr().String(), protocol.GetType(p.Type))

	switch p.Type {
	case 0:
		newStreams := protocol.HelloServer{}
		util.DecodeToStruct(p.Payload, &newStreams)
		if !isMyStream(newStreams.Streams[0].Id, clients) {

			needToSend := false
			newDelta := (uint(time.Now().UnixMicro()) - newStreams.Timestamp) + newStreams.Delta

			for _, newStream := range newStreams.Streams {
				
				needToSend = addToAllStreams(allStreams, newStream, newDelta, strings.Split(conn.RemoteAddr().String(), ":")[0], newStreams.ID)
			}
			// log.Printf("All streams: %v\n", *allStreams)

			if needToSend {
				// Enviar para todos os vizinhos menos o que enviou
				newNeighbours := []string{}
				for _, n := range *neighbours {
					if n != strings.Split(conn.RemoteAddr().String(), ":")[0] {
						newNeighbours = append(newNeighbours, n)
					}
				}
				newStreams.Delta = newDelta

				newP := protocol.NewPacket(0, util.EncodeToBytes(newStreams))

				SendHelloServerMulticast(newNeighbours, newP)
			} else {
				log.Printf("No need to resend HelloServer\n")
			}
		} 
	case 1:
		// TODO
	case 2:
		SendNeighbours(bNeighbours, conn)
	case 3:
		// TODO
	case 4:
		// TODO
	case 5:
		req := protocol.StreamReq{}
		err := util.DecodeToStruct(p.Payload, &req)
		if err != nil {
			log.Printf("Error decoding StreamReq: %v\n", err)
			return
		}
		if isToReq(req.StreamID, allStreams) && !isStreaming(req.StreamID, clients) {
			handleStreamReq(p, conn, allStreams, clients)
		} else {
			cIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			clientAddr, err := net.ResolveUDPAddr("udp", cIP+":"+strconv.Itoa(int(req.PortToSend)))
			if err != nil {
				log.Printf("Error resolving UDP address: %v", err)
				return
			}
			fmt.Println("Adicionar cliente ao stream: ", clientAddr)
			AddRequest(req.StreamID, clients, clientAddr, true)
		}
	case 6:
		// TODO
	case 7:
		// TODO
	case 8:
		handleWantStreams(conn, allStreams)
	case 9:
	case 11:
		stopReq := protocol.StopForwarding{}
		err := util.DecodeToStruct(p.Payload, &stopReq)
		if err != nil {
			log.Printf("Error decoding StopForwarding: %v\n", err)
			return
		}
		// Use the UDPAddress from the stopReq payload
		clientAddr, err := net.ResolveUDPAddr("udp", conn.RemoteAddr().String())
		if err != nil {
			log.Printf("Error resolving UDP address: %v", err)
			return
		}
		// Get Nextop from clients Requests
		newHop := getRequestSender(clients, stopReq.StreamID)
	
		fmt.Printf("Tentar remover este cliente clientaddr.string: %s\n", clientAddr.String())
		RemoveRequest(clientAddr, stopReq.StreamID, clients)
		
		if newHop != "" {

			newConn, err := net.Dial("tcp", newHop+":9090")
			if err != nil {
				log.Printf("Error connecting to next hop: %v\n", err)
				return
			}
			defer newConn.Close()

			newP := util.EncodeToBytes(protocol.NewPacket(11, util.EncodeToBytes(stopReq)))

			_, err = newConn.Write(newP)
			if err != nil {
				log.Printf("Error sending StopForwarding: %v\n", err)
				return
			}
		}
	}
}

func addToAllStreams(allStreams *[]StreamPath, newStream protocol.Stream, delta uint, nextHop string, helloID uuid.UUID) bool {
	r := true
	for i := range *allStreams {
		s := &(*allStreams)[i]
		if s.StreamName == newStream.Name {
			for j := range s.NextHops {
				h := &s.NextHops[j]
				if h.NextHop == nextHop {
					if h.LastHelloID != helloID {
						substituteNextHop(s, nextHop, delta)
						h.LastHelloID = helloID
						return true
					} else {
						r = addHopDeltaBased(s, nextHop, delta)
						return r
					}
				}
			}
			s.NextHops = append(s.NextHops, NextHop{NextHop: nextHop, Delta: delta, LastHelloID: helloID})
			sort.Sort(s.NextHops)
			return true
		}	
	}
	newNextHop := NextHopSlice{{NextHop: nextHop, Delta: delta, LastHelloID: helloID}}
	*allStreams = append(*allStreams, StreamPath{StreamID: newStream.Id, StreamName: newStream.Name, NextHops: newNextHop})

	return true
}

func (a NextHopSlice) Len() int           { return len(a) }
func (a NextHopSlice) Less(i, j int) bool { return a[i].Delta < a[j].Delta }
func (a NextHopSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func substituteNextHop(streamPath *StreamPath, nextHop string, delta uint) {
	for i := range streamPath.NextHops {
		if streamPath.NextHops[i].NextHop == nextHop {
			// fmt.Printf("Substituting next hop %s from delta %d with delta %d\n", nextHop, streamPath.NextHops[i].Delta, delta)
			streamPath.NextHops[i].Delta = delta
			break
		}
	}
}

func addHopDeltaBased(streamPath *StreamPath, nextHop string, delta uint) bool{

	for i := range streamPath.NextHops {
		if streamPath.NextHops[i].NextHop == nextHop {
			if streamPath.NextHops[i].Delta >= delta {
				streamPath.NextHops[i].Delta = delta
				sort.Sort(streamPath.NextHops)
				return true
			} else {
				return false
			}
		}
	}
	
	newHop := NextHop{NextHop: nextHop, Delta: delta}
	streamPath.NextHops = append(streamPath.NextHops, newHop)
	sort.Sort(streamPath.NextHops)

	return streamPath.NextHops[0].NextHop == nextHop
}

func getNextHop(allStreams *[]StreamPath, streamID uuid.UUID) string {
	for _, s := range *allStreams {
		if s.StreamID == streamID {
			return s.NextHops[0].NextHop
		}
	}
	return ""
}

func handleWantStreams(c net.Conn, allStreams *[]StreamPath) {

	availableStreams := protocol.AvailableStreams{
		Streams: allStreamsToStreamList(allStreams),
	}

	responsePacket := protocol.NewPacket(4, util.EncodeToBytes(availableStreams))

	_, err := c.Write(util.EncodeToBytes(responsePacket))
	if err != nil {
		log.Printf("Error sending AvailableStreams: %v", err)
		return
	}
}

func StartListeningUDP(port string, nType string, allStreams *[]StreamPath) {
	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Error initializing UDP server: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening in UDP port %s...\n", port)

	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		go handleUDPPacket(conn, remoteAddr, buffer[:n], nType, allStreams)
	}
}

func handleUDPPacket(conn *net.UDPConn, remoteAddr *net.UDPAddr, data []byte, nType string, allStreams *[]StreamPath) {
	var packet protocol.Packet
	err := util.DecodeToStruct(data, &packet)
	if err != nil {
		log.Printf("Error decoding packet: %v", err)
		return
	}

	switch packet.Type {
	case 6:

	case 9:
		if nType != "p" {
			errorPacket := protocol.Error{Message: "Not a Point of Presence."}

			response := protocol.NewPacket(7, util.EncodeToBytes(errorPacket))

			conn.WriteToUDP(util.EncodeToBytes(response), remoteAddr)

			log.Printf("Sent error to %s\n", remoteAddr.String())
		} else {
			ping := protocol.Ping{}
			err = util.DecodeToStruct(packet.Payload, &ping)
			fmt.Print()
			if err != nil {
				log.Printf("Error while decoding payload: %v", err)
				return
			}
			streamID := ping.StreamID

			HandlePing(conn, remoteAddr, allStreams, streamID)
		}
	default:
		log.Printf("Unexpected type: %s", protocol.GetType(packet.Type))
	}
}

func HandlePing(conn *net.UDPConn, remoteAddr *net.UDPAddr, allStreams *[]StreamPath, streamID uuid.UUID) {

	var bestDelta uint

	for _, s := range *allStreams {
		if s.StreamID == streamID {
			bestDelta = s.NextHops[0].Delta
			break
		}
	}
	pong := protocol.Pong{Delta: bestDelta}

	packet := protocol.NewPacket(10, util.EncodeToBytes(pong))
	responseBytes := util.EncodeToBytes(packet)

	_, err := conn.WriteToUDP(responseBytes, remoteAddr)
	if err != nil {
		log.Printf("Error sending pong: %v", err)
	}
}

/*
Verifica se um neighbour já tem essa stream
*/
func checkIfInStream(allStreams *[]StreamPath, stream protocol.Stream, neighbour string) bool {
	for _, s := range *allStreams {
		if s.StreamName == stream.Name {
			for _, n := range s.NextHops {
				if n.NextHop == neighbour {
					return true
				}
			}
		}
	}
	return false
}

func allStreamsToStreamList(allStreams *[]StreamPath) []protocol.Stream {
	streams := []protocol.Stream{}
	for _, s := range *allStreams {
		streams = append(streams, protocol.Stream{Id: s.StreamID, Name: s.StreamName})
	}
	return streams
}

func handleStreamReq(p protocol.Packet, c net.Conn, allStreams *[]StreamPath, clients *[]Request) {
	log.Printf("Handling StreamReq from %s\n", c.RemoteAddr().String())
	streamReq := protocol.StreamReq{}
	err := util.DecodeToStruct(p.Payload, &streamReq)
	if err != nil {
		log.Printf("Error decoding StreamReq: %v\n", err)
		return
	}

	streamID := streamReq.StreamID

	nextHop := getNextHop(allStreams, streamID)

	if (nextHop == "") {
		log.Printf("Stream not found\n")
		return
	}
	cIP, _, _ := net.SplitHostPort(c.RemoteAddr().String())
	clientAddr, err := net.ResolveUDPAddr("udp", cIP+":"+strconv.Itoa(int(streamReq.PortToSend)))
	if err != nil {
		log.Printf("Error resolving UDP address: %v", err)
		return
	}

	AddRequest(streamID, clients, clientAddr, false)

	newPort := GetReqPort(streamID, clients)

	conn, err := net.Dial("tcp", nextHop+":9090")
	if err != nil {
		log.Printf("Error connecting to next hop: %v\n", err)
		return
	}
	defer conn.Close()

	newReq := protocol.StreamReq{StreamID: streamID, PortToSend: newPort}
	reqPacket := protocol.NewPacket(5, util.EncodeToBytes(newReq))
	_, err = conn.Write(util.EncodeToBytes(reqPacket))
	if err != nil {
		log.Printf("Error sending StreamReq: %v\n", err)
		return
	}
	request := GetRequest(streamID, clients)
	request.sender = nextHop
	go startStreamForwarder(newPort, request)
}
/*
Verifica se n��o é o node que está a fazer o stream
*/
func isToReq(streamID uuid.UUID, allStreams *[]StreamPath) bool {
	for _, s := range *allStreams {
		if s.StreamID == streamID {
			return true
		}
	}
	return false
}

func isStreaming(streamID uuid.UUID, clients *[]Request) bool {
	for i := range *clients {
		if (*clients)[i].streamID == streamID {
			return true
		}
	}
	return false
}

func startStreamForwarder(port uint, request *Request) {

	request.stopChan = make(chan bool)

	log.Println("Starting stream forwarder on port", port)
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprint(":", port))
	if err != nil {
		log.Fatalf("Failed to resolve address: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", port, err)
	}
	defer conn.Close()

	buf := make([]byte, 2048)

	var clientsSnapshot []*net.UDPAddr
	go func() {
		for {
			request.mu.RLock()
			clientsSnapshot = append([]*net.UDPAddr{}, request.clients...)
			fmt.Println("Clients: ", clientsSnapshot)
			request.mu.RUnlock()

			time.Sleep(time.Second)
		}
	}()
	for {
		select {
		case <- request.stopChan:
			log.Printf("Stopping stream forwarder on port %d\n", port)
			return
		default:
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("Error reading from UDP: %v", err)
				continue
			}
			for _, clientAddr := range clientsSnapshot {
				_, err := conn.WriteToUDP(buf[:n], clientAddr)
				if err != nil {
					log.Printf("Error sending to client %s: %v", clientAddr.String(), err)
				}
			}
		}
	}
}

func isMyStream(streamID uuid.UUID, clients *[]Request) bool {
	for i := range *clients {
		if (*clients)[i].streamID == streamID {
			return (*clients)[i].mine
		}
	}
	return false
}



func RemoveRequest(clientAddr *net.UDPAddr, streamID uuid.UUID, clients *[]Request) {
    for i := 0; i < len(*clients); i++ {
        req := &(*clients)[i]
        // Check if the stream ID matches
        if req.streamID == streamID {
            req.mu.Lock() // Lock the request while modifying
            // Iterate through the clients in this request
            for j, addr := range req.clients {
                if addr.IP.String() == clientAddr.IP.String() {
                    // Remove the client from req.clients
                    req.clients = append(req.clients[:j], req.clients[j+1:]...)

					// If there are no more clients and is not the server, cancel the thread
					if len(req.clients) == 0 && !req.mine {
						req.stopChan <- true
						*clients = append((*clients)[:i], (*clients)[i+1:]...)
					}
					req.mu.Unlock() // Unlock after modifying
					return
				}
			}
			req.mu.Unlock() // Unlock if no client was removed
		}
	}

	fmt.Printf("Client %v not found in stream %s\n", clientAddr, streamID)
}

func getRequestSender(clients *[]Request, streamID uuid.UUID) string {
	for i := range *clients {
		if (*clients)[i].streamID == streamID {
			return (*clients)[i].sender
		}
	}
	return ""
}
