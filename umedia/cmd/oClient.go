package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Pdf0/umedia/internal/client"
)

func main() {

	popListFlag := flag.String("pops", "", "POP's list, spearated by comma")

	flag.Parse()

	if *popListFlag == "" {
		log.Fatalln("No POP list specified!")
	}

	pops := strings.Split(*popListFlag, ",")
	fmt.Println("POP list: ", pops)


	streams, err := client.RequestStreamList(pops[0])
	if err != nil {
		log.Fatalf("Error requesting streams list: %v", err)
	}

	if len(streams) == 0 {
		log.Fatalln("No streams available!")
	}

	for i, stream := range streams {
		fmt.Printf("%d | ID: %s | Name: %s\n\n",i , stream.Id, stream.Name)
	}

	var n int
	fmt.Print("Choose the stream number: ")
	fmt.Scanf("%d", &n)
	
	var bestPopIP string
	var bestLatency time.Duration
	if n < 0 || n >= len(streams) {
		log.Fatalf("Invalid stream number: %d", n)
	} else {
		bestPopIP, bestLatency = client.PingPops(pops, streams[n].Id)
	}

	client.SendStreamReq(bestPopIP, streams[n].Id)
	go client.ChangeToBetterPOP(pops, streams[n].Id, bestPopIP, bestLatency)
	client.StartListening(9090)
}
