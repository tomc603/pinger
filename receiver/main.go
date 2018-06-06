package main

import (
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unsafe"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func v4Listener(stopch chan bool, resultchan chan Result, wg *sync.WaitGroup) {
	var stop = false
	defer wg.Done()

	conn, err := icmp.ListenPacket("udp4", "::")
	if err != nil {
		log.Fatalf("FATAL: Error listening on v4 interface: %s\n", err)
	}
	defer conn.Close()

	log.Println("Ping v4Listener running.")
	for {
		if stop {
			break
		}

		err := conn.SetDeadline(time.Now().Add(IODeadline))
		if err != nil {
			log.Fatalf("FATAL: Error setting I/O deadline on v4 interface: %s\n", err)
		}

		select {
		case <-stopch:
			stop = true
			break

		default:
			receiveBuffer := make([]byte, 1500)
			n, peer, err := conn.ReadFrom(receiveBuffer)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			} else if err != nil {
				metrics.Addv4ReceiveFailed(1)
				log.Printf("ERROR: %#v\n", err)
				continue
			}

			receiveMessage, err := icmp.ParseMessage(ProtoICMP, receiveBuffer[:n])
			if err != nil {
				metrics.Addv4ParseFailed(1)
				log.Printf("ERROR: %s\n", err)
			}

			result := Result{
				TimeStamp: time.Now().UnixNano(),
				Address: peer.String(),
			}

			switch receiveMessage.Type {
			case ipv4.ICMPTypeEchoReply:
				metrics.Addv4Received(1)
				metrics.Addv4Bytes(uint(n))

				// TODO: Decode the message, compare the data payload and record the receipt
				// TODO: Decode probe sending location, host, and time from the message payload.
				echoReply := receiveMessage.Body.(*icmp.Echo)
				if ValidateMagic(echoReply.Data[0:unsafe.Sizeof(MagicV1)]) {
					var echoBody Body
					err := echoBody.Decode(echoReply.Data[unsafe.Sizeof(MagicV1):unsafe.Sizeof(echoBody)+1])
					if err == nil {
						result.ReceiveSite = echoBody.Site
						result.ReceiveHost = echoBody.Host
						result.RTT = uint32(time.Unix(0, result.TimeStamp).Sub(time.Unix(0, echoBody.Timestamp)) / time.Millisecond)
					}
				}

				result.ID = uint32(echoReply.ID)
				result.Sequence = uint16(echoReply.Seq)
				result.Code = uint16(receiveMessage.Code)
				result.Type = uint16(receiveMessage.Type.Protocol())

				resultchan <-result
			}
		}
	}
	log.Println("Ping v4Listener stopped.")
}

func v6Listener(stopch chan bool, resultchan chan Result, wg *sync.WaitGroup) {
	var stop = false
	defer wg.Done()

	conn, err := icmp.ListenPacket("udp6", "::")
	if err != nil {
		log.Fatalf("FATAL: Error listening on v6 interface: %s\n", err)
	}
	defer conn.Close()

	log.Println("Ping v6Listener running.")
	for {
		if stop {
			break
		}

		err := conn.SetDeadline(time.Now().Add(IODeadline))
		if err != nil {
			log.Fatalf("FATAL: Error setting I/O deadline on v6 interface: %s\n", err)
		}

		select {
		case <-stopch:
			stop = true
			break

		default:
			receiveBuffer := make([]byte, 1500)
			n, peer, err := conn.ReadFrom(receiveBuffer)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			} else if err != nil {
				metrics.Addv4ReceiveFailed(1)
				log.Printf("ERROR: %#v\n", err)
				continue
			}

			receiveMessage, err := icmp.ParseMessage(ProtoICMPv6, receiveBuffer[:n])
			if err != nil {
				metrics.Addv6ParseFailed(1)
				log.Printf("ERROR: %s\n", err)
			}

			result := Result{
				TimeStamp: time.Now().UnixNano(),
				Address: peer.String(),
			}

			switch receiveMessage.Type {
			case ipv6.ICMPTypeEchoReply:
				metrics.Addv6Received(1)
				metrics.Addv6Bytes(uint(n))

				// TODO: Decode the message, compare the data payload and record the receipt
				// TODO: Decode probe sending location, host, and time from the message payload.
				echoReply := receiveMessage.Body.(*icmp.Echo)
				result.ID = uint32(echoReply.ID)
				result.Sequence = uint16(echoReply.Seq)
				result.Code = uint16(receiveMessage.Code)
				result.Type = uint16(receiveMessage.Type.Protocol())

				resultchan <-result
			}
		}
	}
	log.Println("Ping v6Listener stopped.")
}

func resultWriter(resultchan chan Result, sqldb *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()

	for result := range resultchan {
		log.Printf("%s\n", result.String())
		//err := result.Commit(sqldb)
		//if err != nil {
		//	log.Printf("ERROR: Would not commit result %v.\n", result)
		//}
	}
}

var metrics = new(Metrics)

func main() {
	var stop = false

	receiveWG := sync.WaitGroup{}
	resultWG := sync.WaitGroup{}

	resultch := make(chan Result, 100)
	sigch := make(chan os.Signal, 5)
	stopch := make(chan bool)

	signal.Notify(sigch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGUSR1)

	metrics.Lock()
	metrics.startTime = time.Now()
	metrics.Unlock()

	sqldb, err := sql.Open("sqlite3", "./db.sqlite3")
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
	}
	defer sqldb.Close()

	// sources := db.GetSources(sqldb)
	// destinations := db.GetDestinations(sqldb)

	resultWG.Add(1)
	go resultWriter(resultch, sqldb, &resultWG)

	receiveWG.Add(2)
	go v6Listener(stopch, resultch, &receiveWG)
	go v4Listener(stopch, resultch, &receiveWG)

	statsTicker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-statsTicker.C:
			log.Printf("%s\n", metrics)
		case s := <-sigch:
			log.Printf("Received signal %s.\n", s)
			switch s {
			case syscall.SIGUSR1:
				log.Printf("%s\n", metrics)
			default:
				stop = true
			}
			break
		}

		if stop {
			statsTicker.Stop()
			break
		}
	}

	// Tell the receiver functions to stop, and wait for them.
	close(stopch)
	receiveWG.Wait()

	// Tell the result processor function to stop, and wait for it.
	close(resultch)
	resultWG.Wait()

	log.Printf("Exiting ping receiver.")
}
