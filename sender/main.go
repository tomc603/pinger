package main

import (
	"bytes"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tomc603/pinger/data"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// TODO: Make StatsInterval a config parameter.
const StatsInterval = 60
// TODO: Make SenderID & SiteID config parameters. They should match values in the Senders table.
const SenderID    = 121
const SiteID      = 101
// TODO: Make DbPath a DSN, and a config parameter.
const DbPath = "/Users/tcameron/pinger.sqlite3"

// TODO: Add functions for other types of probe than ICMP.

func ping(destinations chan *data.Destination, stopch chan bool, wg *sync.WaitGroup) {
	var stop = false
	var seq = 0
	wg.Add(1)
	defer wg.Done()

	// Setup connections so we aren't constantly creating and tearing them down
	// This probably doesn't save much overhead, but on a busy system it's easy
	// to imagine running out of descriptors, ports, or both.
	v6conn, err := icmp.ListenPacket("udp6", "::")
	if err != nil {
		log.Fatal(err)
	}

	v4conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Ping sender running.")
	for {
		if stop {
			break
		}

		select {
		case dest := <-destinations:
			var v6 = false
			var listenNetType = "ip4"
			conn := v4conn

			if dest == nil || dest.Address == "" || dest.Protocol == 0 {
				// Because we've closed the channel, the pointer to a Destination could be a nil pointer
				// or the Destination could be empty/meaningless due to data error.
				metrics.AddEmptyDest(1)
				log.Println("Received an empty Destination")
				continue
			}

			//log.Printf("Received dispatched destination: %v\n", dest)

			// TODO: Prepend probe sending location, host, and time into the message payload.
			bodyBuffer := new(bytes.Buffer)
			magicData, magicErr := data.MagicV1.Encode()
			if magicErr != nil {
				log.Printf("WARN: Could not encode Magic value. %s\n", magicErr)
			}
			bodyBuffer.Write(magicData)

			body := data.Body{
				Timestamp: time.Now().UnixNano(),
				Site: SiteID,
				Host: SenderID,
			}
			bodyData, bodyErr := body.Encode()
			if bodyErr != nil || magicErr != nil {
				bodyBuffer.Reset()
				log.Printf("WARN: Skipping diagnostic payload.")
			} else {
				bodyBuffer.Write(bodyData)
			}

			bodyBuffer.Write(dest.Data)
			echoRequestBody := icmp.Echo{
				ID:   SenderID<<8 | 0x0000 + 1,
				Seq:  seq,
				Data: bodyBuffer.Bytes(),
			}
			echoRequestMessage := icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &echoRequestBody,
			}

			if dest.Protocol == data.ProtoUDP6 {
				v6 = true
				conn = v6conn
				listenNetType = "ip6"
				echoRequestMessage.Type = ipv6.ICMPTypeEchoRequest
			}

			destAddr, err := net.ResolveIPAddr(listenNetType, dest.Address)
			if err != nil {
				switch err.(type) {
				case *net.DNSError:
					e := err.(*net.DNSError)
					if e.IsTimeout {
						metrics.AddDnsTimeout(1)
						log.Printf("ERROR: DNS Timeout: %#v", e.Name)
					} else if e.IsTemporary {
						metrics.AddDnsTempFail(1)
						log.Printf("ERROR: DNS Temporary Failure: %#v", e)
					} else {
						metrics.AddDnsError(1)
						log.Printf("ERROR: DNS Error: %#v", e)
					}
				case *net.AddrError:
					metrics.AddAddressError(1)
					log.Printf("ERROR: No %s Address: '%s'. %s",
						listenNetType,
						err.(*net.AddrError).Addr, err.(*net.AddrError).Err)
				default:
					metrics.AddUnknownError(1)
					log.Printf("ERROR: Unexpected error: %#v", err)
				}
				continue
			}

			//log.Printf("Address: %v", destAddr)
			echoRequest, err := echoRequestMessage.Marshal(nil)
			if err != nil {
				log.Fatal(err)
			}

			b, err := conn.WriteTo(echoRequest, &net.UDPAddr{IP: destAddr.IP})
			//dest.Last = time.Now().UnixNano()
			if err != nil {
				if v6 {
					metrics.Addv6Failed(1)
				} else {
					metrics.Addv4Failed(1)
				}
				log.Printf("ERROR: %s", err)
				continue
			} else {
				if v6 {
					metrics.Addv6Sent(1)
					metrics.Addv6Bytes(uint(b))
				} else {
					metrics.Addv4Sent(1)
					metrics.Addv4Bytes(uint(b))
				}
			}

			seq += 1

		case <-stopch:
			stop = true
			break
		}
	}
	log.Println("Name channel closed.")
}

var metrics = new(Metrics)

func main() {
	var stop = false

	destWG := sync.WaitGroup{}
	pingWG := sync.WaitGroup{}

	sigch := make(chan os.Signal, 5)
	namech := make(chan *data.Destination, 100)
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

	// TODO: Put this in a Ticker controlled coroutine to watch for new, removed, and modified Destinations
	destinations := data.GetDestinations(sqldb)
	//destinations := []*data.Destination{
	//	//{Address: "www.reddit.com", Protocol: data.ProtoUDP4, Interval: 1500, Data: []byte("TeStDaTA"), Active:true},
	//	////{Address: "google-public-dns-a.google.com", Protocol: data.ProtoUDP6, Interval: 1000, Data: []byte("TesTdaTa"), Active:true},
	//	////{Address: "www.google.com", Protocol: data.ProtoUDP6, Interval: 750, Data: []byte("TEsTDaTA"), Active:true},
	//	//{Address: "www.yahoo.com", Protocol: data.ProtoUDP4, Interval: 5000, Data: []byte("teSTdaTA"), Active:true},
	//	//{Address: "www.amazon.com", Protocol: data.ProtoUDP4, Interval: 10000, Data: []byte("tEsTdATa"), Active:true},
	//	//{Address: "1.1.1.1", Protocol: data.ProtoUDP4, Interval: 2000, Data: []byte("tEsTdATa"), Active:false},
	//	{Address: "localhost", Protocol: data.ProtoUDP4, Interval: 500, Data: []byte("tEsTdATa"), Active:true},
	//	{Address: "::1", Protocol: data.ProtoUDP6, Interval: 500, Data: []byte("tEsTdATa"), Active:true},
	//}
	if len(destinations) == 0 {
		// TODO: Remove this check once we start watching for new DB entries
		log.Fatal("ERROR: no destinations to check")
	}

	statsTicker := &time.Ticker{}
	if StatsInterval > 0 {
		statsTicker = time.NewTicker(StatsInterval * time.Second)
	}

	go ping(namech, stopch, &pingWG)

	for _, destination := range destinations {
		destination.Start(namech, stopch, &destWG)
	}

	for {
		if stop {
			for _, destination := range destinations {
				destination.Stop()
			}
			statsTicker.Stop()
			break
		}

		select {
		case <-statsTicker.C:
			log.Printf("%s\n", metrics)
		case s := <-sigch:
			switch s {
			// Handle any type of quit/kill/term/abort signal the same
			case syscall.SIGHUP:
				fallthrough
			case syscall.SIGINT:
				fallthrough
			case syscall.SIGQUIT:
				fallthrough
			case syscall.SIGABRT:
				fallthrough
			case syscall.SIGKILL:
				log.Printf("Received signal %s.\n", s)
				stop = true
			// If a user sends SIGUSR1, output the current metrics values.
			case syscall.SIGUSR1:
				log.Printf("%s\n", metrics)
			}
			break
		}
	}

	// Tell the destination routine(s) that we're finished, and wait for a graceful stop.
	close(stopch)
	close(namech)
	destWG.Wait()
	pingWG.Wait()
	log.Printf("Exiting ping sender.")
}
