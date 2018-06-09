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
	"github.com/tomc603/pinger/data"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// TODO: Make StatsInterval a config parameter.
const StatsInterval = 60
// TODO: Make ResultBatchSize a config parameter. If 0, do not batch results.
const ResultBatchSize = 10
// TODO: Make DbPath a DSN, and a config parameter.
const DbPath = "/Users/tcameron/pinger.sqlite3"

func v4Listener(stopch chan bool, resultchan chan data.Result, wg *sync.WaitGroup) {
	var stop = false

	wg.Add(1)
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

		err := conn.SetDeadline(time.Now().Add(data.IODeadline))
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

			receiveMessage, err := icmp.ParseMessage(data.ProtoICMP, receiveBuffer[:n])
			if err != nil {
				metrics.Addv4ParseFailed(1)
				log.Printf("ERROR: %s\n", err)
			}

			result := data.Result{
				TimeStamp: time.Now().UnixNano(),
				Address: peer.String(),
			}

			switch receiveMessage.Type {
			case ipv4.ICMPTypeEchoReply:
				metrics.Addv4Received(1)
				metrics.Addv4Bytes(uint(n))

				// TODO: Compare the data payload and record match / no match in the receipt
				echoReply := receiveMessage.Body.(*icmp.Echo)

				// Magic number means we have embedded data
				if data.ValidateMagic(echoReply.Data[0:unsafe.Sizeof(data.MagicV1)]) {
					var echoBody data.Body
					err := echoBody.Decode(echoReply.Data[unsafe.Sizeof(data.MagicV1):unsafe.Sizeof(echoBody)+1])
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

func v6Listener(stopch chan bool, resultchan chan data.Result, wg *sync.WaitGroup) {
	var stop = false

	wg.Add(1)
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

		err := conn.SetDeadline(time.Now().Add(data.IODeadline))
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
				log.Printf("ERROR: reading from connection to buffer. %s\n", err)
				continue
			}

			receiveMessage, err := icmp.ParseMessage(data.ProtoICMPv6, receiveBuffer[:n])
			if err != nil {
				metrics.Addv6ParseFailed(1)
				log.Printf("ERROR: parsing ICMP message. %s\n", err)
			}

			result := data.Result{
				TimeStamp: time.Now().UnixNano(),
				Address: peer.String(),
			}

			switch receiveMessage.Type {
			case ipv6.ICMPTypeEchoReply:
				metrics.Addv6Received(1)
				metrics.Addv6Bytes(uint(n))

				// TODO: Compare the data payload and record match / no match in the receipt
				echoReply := receiveMessage.Body.(*icmp.Echo)

				// Magic number means we have embedded data
				if data.ValidateMagic(echoReply.Data[0:unsafe.Sizeof(data.MagicV1)]) {
					var echoBody data.Body
					err := echoBody.Decode(echoReply.Data[unsafe.Sizeof(data.MagicV1):unsafe.Sizeof(echoBody)+1])
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
	log.Println("Ping v6Listener stopped.")
}

func resultWriter(resultchan chan data.Result, sqldb *sql.DB, wg *sync.WaitGroup) {
	var resultBuf []*data.Result

	wg.Add(1)
	defer wg.Done()

	log.Println("Ping resultWriter started.")
	for result := range resultchan {
		//log.Printf("%s\n", result.String())

		if ResultBatchSize == 0 {
			if ce := result.Commit(sqldb); ce != nil {
				log.Printf("ERROR: Could not commit Result %#v. %s.\n", result, ce)
				metrics.AddDbFailedSingleCommits(1)
			} else {
				metrics.AddDbSingleCommits(1)
			}
		} else if len(resultBuf) >= ResultBatchSize {
			if err := data.BatchResultWriter(resultBuf, sqldb); err != nil {
				// Commit each Result individually so we save as much data as possible.
				log.Printf("ERROR: Could not commit Result batch. %s.\n", err)
				metrics.AddDbFailedBatchCommits(1)
				for _, r := range resultBuf {
					if ce := r.Commit(sqldb); ce != nil {
						log.Printf("ERROR: Could not commit Result %#v. %s.\n", r, ce)
						metrics.AddDbFailedSingleCommits(1)
					} else {
						metrics.AddDbSingleCommits(1)
					}
				}
			} else {
				metrics.AddDbBatchCommits(1)
			}

			// Empty the result buffer.
			resultBuf = []*data.Result{}
		} else {
			resultBuf = append(resultBuf, &result)
		}
	}
	log.Println("Ping resultWriter stopped.")
}

var metrics = new(Metrics)

func main() {
	var stop = false

	receiveWG := sync.WaitGroup{}
	resultWG := sync.WaitGroup{}

	resultch := make(chan data.Result, 100)
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

	sqldb, err := sql.Open("sqlite3", DbPath)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
	}
	defer sqldb.Close()

	// Create tables if they don't exist.
	if err := data.CreateSourcesTable(sqldb); err != nil {
		log.Fatalf("ERROR: Sources table could not be created. %s.\n", err)
	}
	if err := data.CreateDestinationsTable(sqldb); err != nil {
		log.Fatalf("ERROR: Destinations table could not be created. %s.\n", err)
	}
	if err := data.CreateResultsTable(sqldb); err != nil {
		log.Fatalf("ERROR: Results table could not be created. %s.\n", err)
	}

	// sources := db.GetSources(sqldb)

	statsTicker := &time.Ticker{}
	if StatsInterval > 0 {
		statsTicker = time.NewTicker(StatsInterval * time.Second)
	}

	go resultWriter(resultch, sqldb, &resultWG)
	go v6Listener(stopch, resultch, &receiveWG)
	go v4Listener(stopch, resultch, &receiveWG)

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

	// Wait until all of the receivers have stopped and returned,
	// then stop the result processor. There shouldn't be a race here,
	// but I'm being cautious nonetheless.
	close(resultch)
	resultWG.Wait()

	log.Printf("Exiting ping receiver.")
}
