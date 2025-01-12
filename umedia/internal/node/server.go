package node

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"strconv"

	"github.com/Pdf0/umedia/internal/protocol"
	"github.com/Pdf0/umedia/internal/util"
	"github.com/google/uuid"
)

type Request struct {
    streamID uuid.UUID
    clients  []*net.UDPAddr
	mu 	     sync.RWMutex
    port     uint
    mine     bool
    sender   string
    stopChan chan bool
}

func SendHelloServerMulticast(neighbours []string, packetToSend protocol.Packet) {
	for _, n := range neighbours {
		// log.Printf("Sending HelloServer to %s\n", n)
		go sendStreamAux(n, packetToSend)
	}
}

func sendStreamAux (n string, packetToSend protocol.Packet) {
    payload := protocol.HelloServer{}
    util.DecodeToStruct(packetToSend.Payload, &payload)

    payload.Timestamp = uint(time.Now().UnixMicro())

    packetToSend.Payload = util.EncodeToBytes(payload)

	conn, err :=  net.Dial("tcp", n + ":9090", )
	if err != nil {
		log.Printf("Failed to connect to %s: %v\n", n, err)
        return
	}
	defer conn.Close()

	conn.Write(util.EncodeToBytes(packetToSend))
}

func GetServerStreams(streamsNames *string) []protocol.Stream {
	streams := []protocol.Stream{}

	for _, streamName := range strings.Split(*streamsNames, ",") {

		checkIfFileExists("videos/" + streamName)

		streams = append(streams, protocol.Stream{ Id: uuid.New(), Name: streamName})
	}
	return streams
}

func checkIfFileExists(streamName string) {	
	file, err := os.Open(streamName)
    if err != nil {
        log.Fatalf("Failed to open file: %s\nPlace it in the 'videos' folder", err)
    }
	file.Close()
}

func StreamVideos(streams []protocol.Stream, requests *[]Request) {
    port := 9091
	for _, stream := range streams {
        for i := range *requests {
            request := &(*requests)[i]
            if stream.Id == request.streamID {
                go Stream(stream, strconv.Itoa(port), request)
                port++
            }
        }
	}
}

func Stream(stream protocol.Stream, port string, clients *Request) {
    addr, err := net.ResolveUDPAddr("udp", fmt.Sprint(":", port))
    if err != nil {
        log.Fatalf("Failed to resolve UDP address: %v", err)
    }

	conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        log.Fatalf("Failed to listen on UDP: %v", err)
    }
    defer conn.Close()

	videoFile := "videos/" + stream.Name
    
    ffmpegCmd := exec.Command(
        "ffmpeg",
        "-re",
        "-stream_loop", "-1",
        "-i", videoFile,
        "-c:v", "libx264",
        "-preset", "ultrafast",
        "-tune", "zerolatency",
        "-b:v", "1500k",
        "-maxrate", "1500k",
        "-bufsize", "3000k",
        "-g", "50",
        "-f", "mpegts",
        "-pkt_size", "1316",
        "pipe:1",
    )
    ffmpegStdout, err := ffmpegCmd.StdoutPipe()
    if err != nil {
        log.Fatalf("Failed to get ffmpeg stdout: %v", err)
    }
    if err := ffmpegCmd.Start(); err != nil {
        log.Fatalf("Failed to start ffmpeg: %v", err)
    }
    log.Println("ffmpeg started streaming video" + stream.Name +" on port " + port)


    reader := bufio.NewReader(ffmpegStdout)
    buf := make([]byte, 1316)

    var clientsSnapshot []*net.UDPAddr
    go func() {
        for {
			clients.mu.RLock()
			clientsSnapshot = append([]*net.UDPAddr{}, clients.clients...)
            // fmt.Printf("%s's", stream.Name)
            // fmt.Println(" clients: ", clientsSnapshot)
			clients.mu.RUnlock()


			time.Sleep(time.Second)
		}
	}()

    for {
        n, err := reader.Read(buf)
        if err != nil {
            log.Printf("Error reading ffmpeg stdout: %v", err)
            break
        }

        for _, clientAddr := range clientsSnapshot {
            _, err := conn.WriteToUDP(buf[:n], clientAddr)
            if err != nil {
                log.Printf("Error sending to client %s: %v", conn.RemoteAddr().String(), err)
            }
        }
    }
    ffmpegCmd.Wait()
}

func FillRequests(streams []protocol.Stream, clients *[]Request) []Request {
    for _, stream := range streams {
        *clients = append(*clients, Request{streamID: stream.Id, clients: []*net.UDPAddr{}, port: 9091, mine: true})
    }
    return *clients
}

func AddRequest(streamID uuid.UUID, clients *[]Request, client *net.UDPAddr, mine bool) {
    for i := range *clients {
        request := &(*clients)[i]
        if request.streamID == streamID {
            request.mu.Lock()
            request.clients = append(request.clients, client)
            request.mu.Unlock()
            fmt.Println("Client added to stream")
            return
        }
    }

    if len(*clients) == 0 {
        *clients = append(*clients, Request{streamID: streamID, clients: []*net.UDPAddr{client}, port: 9091})
        return
    } else{
        newPort := (*clients)[len(*clients)-1].port + 1
        fmt.Printf("Streaming new stream on port %d\n", newPort)
        *clients = append(*clients, Request{streamID: streamID, clients: []*net.UDPAddr{client}, port: newPort})
    }
}

func GetRequest(streamID uuid.UUID, clients *[]Request) *Request {
    for i := range *clients {
        request := &(*clients)[i]
        if request.streamID == streamID {
            return request
        }
    }
    return &Request{}
}


func GetReqPort(streamID uuid.UUID, clients *[]Request) uint {
    for i := range *clients {
        request := &(*clients)[i]
        if request.streamID == streamID {
            return request.port
        }
    }
    return 0
}
