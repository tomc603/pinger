package main

import (
	"bytes"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	SenderID    = 121
)

func ping(destinations chan *Destination, stopch chan bool, wg *sync.WaitGroup) {
	var stop = false
	var seq = 0
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
			magicData, magicErr := MagicV1.Encode()
			if magicErr != nil {
				log.Printf("WARN: Could not encode Magic value. %s\n", magicErr)
			}
			bodyBuffer.Write(magicData)

			body := Body{
				Timestamp: time.Now().UnixNano(),
				Site: 101,
				Host: 62,
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

			if dest.Protocol == ProtoUDP6 {
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
			dest.Last = time.Now().UnixNano()
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
	metrics.Lock()
	metrics.startTime = time.Now()
	metrics.Unlock()

	destinations := []*Destination{
		{Address: "www.reddit.com", Protocol: ProtoUDP4, Interval: 1500, Data: []byte("TeStDaTA"), Active:true},
		//{Address: "google-public-dns-a.google.com", Protocol: ProtoUDP6, Interval: 1000, Data: []byte("TesTdaTa"), Active:true},
		//{Address: "www.google.com", Protocol: ProtoUDP6, Interval: 750, Data: []byte("TEsTDaTA"), Active:true},
		{Address: "www.yahoo.com", Protocol: ProtoUDP4, Interval: 5000, Data: []byte("teSTdaTA"), Active:true},
		{Address: "www.amazon.com", Protocol: ProtoUDP4, Interval: 10000, Data: []byte("tEsTdATa"), Active:true},
		{Address: "1.1.1.1", Protocol: ProtoUDP4, Interval: 2000, Data: []byte("tEsTdATa"), Active:false},
		{Address: "localhost", Protocol: ProtoUDP4, Interval: 500, Data: []byte("tEsTdATa"), Active:true},
		//{Address: "::1", Protocol: ProtoUDP6, Interval: 500, Data: []byte("tEsTdATa"), Active:true},
	}

	sigch := make(chan os.Signal, 5)
	namech := make(chan *Destination, 100)
	stopch := make(chan bool)

	signal.Notify(sigch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGUSR1)

	pingWG := sync.WaitGroup{}
	pingWG.Add(1)
	go ping(namech, stopch, &pingWG)

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
		default:
			for _, destination := range destinations {
				if time.Now().UnixNano() > destination.Last + (int64(time.Millisecond) * int64(destination.Interval)) {
					// Slight hack to prevent the loop from attempting to send again immediately after dispatch
					// The value will be updated again after a transmit attempt is made
					destination.Last = time.Now().UnixNano()
					namech <- destination
				}
			}
		}

		if stop {
			statsTicker.Stop()
			break
		}
	}

	// Tell the ping routine(s) that we're finished, and wait for a graceful stop.
	close(stopch)
	close(namech)
	pingWG.Wait()
	log.Printf("Exiting ping sender.")
}
