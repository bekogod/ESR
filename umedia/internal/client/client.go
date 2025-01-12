package client

import (
	"fmt"
	"net"
	"time"
	"os/exec"
	"log"

	"github.com/Pdf0/umedia/internal/protocol"
	"github.com/Pdf0/umedia/internal/util"
	"github.com/google/uuid"
)

func RequestStreamList(popAddress string) ([]protocol.Stream, error) {
	conn, err := net.Dial("tcp", popAddress+":9090")
	if err != nil {
		return nil, fmt.Errorf("error connecting to POP: %v", err)
	}
	defer conn.Close()

	wantStreams := protocol.WantStreams{
		ClientIP: conn.LocalAddr().String(),
	}

	requestPacket := protocol.NewPacket(8, util.EncodeToBytes(wantStreams))
	_, err = conn.Write(util.EncodeToBytes(requestPacket))
	if err != nil {
		return nil, fmt.Errorf("error sending the request: %v", err)
	}

	// Receber a resposta
	responseBytes := make([]byte, 4096)
	n, err := conn.Read(responseBytes)
	if err != nil {
		return nil, fmt.Errorf("error receiving the answer: %v", err)
	}

	// Desserializar a resposta
	responsePacket := protocol.Packet{}
	err = util.DecodeToStruct(responseBytes[:n], &responsePacket)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %v", err)
	}

	// Verificar se a resposta é do tipo AvailableStreams
	if responsePacket.Type != 4 {
		return nil, fmt.Errorf("unexpected type: %s", protocol.GetType(responsePacket.Type))
	}

	// Desserializar o payload para AvailableStreams
	availableStreams := protocol.AvailableStreams{}
	err = util.DecodeToStruct(responsePacket.Payload, &availableStreams)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %v", err)
	}

	return availableStreams.Streams, nil
}

func PingPops(pops []string, streamID uuid.UUID) (string, time.Duration) {
    bestPop := ""
    bestLatency := time.Duration(1<<63 - 1)

    for _, pop := range pops {
        latency, err := pingPop(pop, streamID)
        if err != nil {
            fmt.Printf("Error pinging POP %s: %v\n", pop, err)
            continue
        }

        fmt.Printf("POP %s - Latency: %v\n", pop, latency)

        if latency < bestLatency {
            bestLatency = latency
            bestPop = pop
        }
    }

    if bestPop != "" {
        fmt.Printf("Best POP: %s with a latency of %v\n", bestPop, bestLatency)
    } else {
        fmt.Println("No POP available")
    }
    // IP do melhor POP
    return bestPop, bestLatency
}

func pingPop(popAddress string, streamID uuid.UUID) (time.Duration, error) {
	addr, err := net.ResolveUDPAddr("udp", popAddress+":9090")
	if (err != nil) {
		return 0, fmt.Errorf("error resolving UDP address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return 0, fmt.Errorf("error connecting to POP: %v", err)
	}
	defer conn.Close()

	pingPacket := protocol.NewPacket(9, util.EncodeToBytes(protocol.Ping{StreamID: streamID}))
	packetBytes := util.EncodeToBytes(pingPacket)

	_, err = conn.Write(packetBytes)
	if err != nil {
		return 0, fmt.Errorf("error pinging: %v", err)
	}
	start := time.Now()

	conn.SetReadDeadline(time.Now().Add(time.Second))
	responseBytes := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(responseBytes)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, fmt.Errorf("timeout waiting for POP pong")
		}
		return 0, fmt.Errorf("error receiving pong: %v", err)
	}

	latency := time.Since(start)

    dPacket := protocol.Packet{}

    err = util.DecodeToStruct(responseBytes[:n], &dPacket)
    if err != nil {
        return 0, fmt.Errorf("error decoding payload: %v", err)
    }

	pong := protocol.Pong{}
	err = util.DecodeToStruct(dPacket.Payload, &pong)
	if err != nil {
		return 0, fmt.Errorf("error decoding payload: %v", err)
	}

	latency += latency + time.Duration(pong.Delta)

	responsePacket := protocol.Packet{}
	err = util.DecodeToStruct(responseBytes[:n], &responsePacket)
	if err != nil {
		return 0, fmt.Errorf("error decoding payload: %v", err)
	}

	if responsePacket.Type != 10 {
		return 0, fmt.Errorf("unexpected type: %s", protocol.GetType(responsePacket.Type))
	}

	return latency, nil
}

func SendStreamReq(ip string, streamID uuid.UUID) error {
	conn, err := net.Dial("tcp", ip+":9090")
	if err != nil {
		return fmt.Errorf("error connecting to POP: %v", err)
	}
	defer conn.Close()

	reqPacket := protocol.NewPacket(5, util.EncodeToBytes(protocol.StreamReq{StreamID: streamID, PortToSend: 9090}))
	_, err = conn.Write(util.EncodeToBytes(reqPacket))
	if err != nil {
		return fmt.Errorf("error sending the request: %v", err)
	}

	return nil
}

func StartListening(port uint) {
	time.Sleep(1*time.Second)
	ffplayCmd := exec.Command(
		"ffplay",
		"-fflags", "nobuffer",
		"-analyzeduration", "100000",
		"-probesize", "500000",
		"-flags", "low_delay",
		"-framedrop",
		"-sync", "ext",
		"-i", fmt.Sprintf("udp://@:%d", port),
		"-autoexit",
		"-hide_banner",
		"-loglevel", "error",
	)

	//ffplayCmd.Stderr = log.Writer()

	//ffplayCmd.Stdout = log.Writer()


    log.Printf("Starting ffplay to listen on UDP port %d", port)
    if err := ffplayCmd.Start(); err != nil {
        log.Fatalf("Failed to start ffplay: %v", err)
    }

    ffplayCmd.Wait()
}


func ChangeToBetterPOP(pops []string, streamID uuid.UUID, currentPop string, currentLatency time.Duration) {
    const checkInterval = 5 * time.Second
    const improvementThreshold = 0.15 // 15% improvement as a float64
    const consecutiveThreshold = 2  // 3 consecutive improvements

    consecutiveCount := make(map[string]int) // Track consecutive improvements per POP
    bestPop := currentPop                    // Keep track of the current best POP
    bestLatency := currentLatency            // Best latency corresponds to the best POP
    ticker := time.NewTicker(checkInterval)

	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("Checking for better POP...\n")
		for _, pop := range pops {
			fmt.Printf("Comparing pop %s with bestPop %s\n", pop, bestPop)

			latency, err := pingPop(pop, streamID)
			if err != nil {
				fmt.Printf("Error pinging POP %s: %v\n", pop, err)
				continue
			}

			fmt.Printf("POP %s - Latency: %v\n", pop, latency)

			if pop == bestPop {
				bestLatency = latency
				continue
			}

			// Convert durations to float64 for calculation
			if float64(latency) < float64(bestLatency) && 
			   (float64(bestLatency-latency) >= float64(bestLatency)*improvementThreshold) {
				consecutiveCount[pop]++
				fmt.Printf("POP %s latency improved significantly (%d/3 checks)\n", pop, consecutiveCount[pop])
				if consecutiveCount[pop] >= consecutiveThreshold {
					fmt.Printf("Switching to POP %s with latency %v (improved over %v)\n", pop, latency, bestLatency)

					NotifyCurrentPopToStop(bestPop, streamID)

					time.Sleep(2 * time.Second)

					// Update the best POP and latency
					bestPop = pop
					bestLatency = latency

					// Reset the improvement counters
					consecutiveCount = make(map[string]int)

					// Send a stream request to the new POP
					err := SendStreamReq(bestPop, streamID)
					if err != nil {
						log.Printf("Error sending stream request to POP %s: %v\n", bestPop, err)
					}

					break
				}
			} else {
				// Reset count for the POP if it fails to improve
				consecutiveCount[pop] = 0
			}
		}
	}
}




func NotifyCurrentPopToStop(currentPop string, streamID uuid.UUID) {
    conn, err := net.Dial("tcp", currentPop+":9090")
    if err != nil {
        log.Printf("Error connecting to current POP: %v", err)
        return
    }
    defer conn.Close()

    // Update the protocol to include UDP address in the request payload
    stopRequest := protocol.StopForwarding{
        StreamID:    streamID,
    }

    stopPacket := protocol.NewPacket(11, util.EncodeToBytes(stopRequest))
    _, err = conn.Write(util.EncodeToBytes(stopPacket))
    if err != nil {
        log.Printf("Error sending stop request: %v", err)
    }
}


