package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ProtoICMP   = 1
	ProtoICMPv6 = 58
	SenderID    = 121
)

type DestinationMetrics struct {
	v4Sent       uint
	v4Failed     uint
	v4Bytes      uint
	v6Sent       uint
	v6Failed     uint
	v6Bytes      uint
	dnsError     uint
	addrError    uint
}

type Metrics struct {
	sync.RWMutex
	v4Sent       uint
	v4Failed     uint
	v4Bytes      uint
	v6Sent       uint
	v6Failed     uint
	v6Bytes      uint
	emptyDest    uint
	dnsTimeout   uint
	dnsTempFail  uint
	dnsError     uint
	addrError    uint
	unknownError uint
	startTime    time.Time
	destMetrics  map[string]DestinationMetrics
}

type Destination struct {
	name     string
	protocol string
	data     []byte
	interval int64
	last     time.Time
}

func (m *Metrics) Addv4Sent(i uint) {
	m.Lock()
	m.v4Sent += i
	m.Unlock()
}

func (m *Metrics) Addv4Failed(i uint) {
	m.Lock()
	m.v4Failed += i
	m.Unlock()
}

func (m *Metrics) Addv4Bytes(i uint) {
	m.Lock()
	m.v4Bytes += i
	m.Unlock()
}

func (m *Metrics) Addv6Sent(i uint) {
	m.Lock()
	m.v6Sent += i
	m.Unlock()
}

func (m *Metrics) Addv6Failed(i uint) {
	m.Lock()
	m.v6Failed += i
	m.Unlock()
}

func (m *Metrics) Addv6Bytes(i uint) {
	m.Lock()
	m.v6Bytes += i
	m.Unlock()
}

func (m *Metrics) AddEmptyDest(i uint) {
	m.Lock()
	m.emptyDest += i
	m.Unlock()
}

func (m *Metrics) AddDnsTimeout(i uint) {
	m.Lock()
	m.dnsTimeout += i
	m.Unlock()
}

func (m *Metrics) AddDnsTempFail(i uint) {
	m.Lock()
	m.dnsTempFail += i
	m.Unlock()
}

func (m *Metrics) AddDnsError(i uint) {
	m.Lock()
	m.dnsError += i
	m.Unlock()
}

func (m *Metrics) AddAddressError(i uint) {
	m.Lock()
	m.addrError += i
	m.Unlock()
}

func (m *Metrics) AddUnknownError(i uint) {
	m.Lock()
	m.unknownError += i
	m.Unlock()
}

func (m *Metrics) String() string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("Uptime: %v\n" +
		"IPv4 sent: %d\n" +
		"IPv4 failed: %d\n" +
		"IPv4 bytes: %d\n" +
		"IPv6 sent: %d\n" +
		"IPv6 failed: %d\n" +
		"IPv6 bytes: %d\n" +
		"Total sent: %d\n" +
		"Total failed: %d\n" +
		"Total bytes: %d\n" +
		"Empty Destination: %d\n" +
		"DNS timeouts: %d\n" +
		"DNS temporary failures: %d\n" +
		"DNS errors: %d\n" +
		"Address errors: %d\n" +
		"Unknown errors: %d\n",
		time.Since(m.startTime),
		m.v4Sent, m.v4Failed, m.v4Bytes,
		m.v6Sent, m.v6Failed, m.v6Bytes,
		m.v4Sent+m.v6Sent, m.v4Failed + m.v6Failed, m.v4Bytes + m.v6Bytes,
		m.emptyDest, m.dnsTimeout, m.dnsTempFail, m.dnsError,
		m.addrError, m.unknownError)
}

func ping(destinations chan *Destination, stopch chan bool, wg *sync.WaitGroup) {
	var stop bool = false
	var seq int = 0
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
			var v6 bool = false
			var listenNetType string = "ip4"
			conn := v4conn

			if dest == nil || dest.name == "" || dest.protocol == "" {
				// Because we've closed the channel, the pointer to a Destination could be a nil pointer
				// or the Destination could be empty/meaningless due to data error.
				metrics.AddEmptyDest(1)
				log.Println("Received an empty Destination")
				continue
			}

			//log.Printf("Received dispatched destination: %v\n", dest)

			// TODO: Prepend probe sending location, host, and time into the message payload.
			echoRequestBody := icmp.Echo{
				ID:   SenderID<<8 | 0x0000 + 1,
				Seq:  seq,
				Data: dest.data,
			}
			echoRequestMessage := icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &echoRequestBody,
			}

			if strings.HasSuffix(dest.protocol, "6") {
				v6 = true
				conn = v6conn
				listenNetType = "ip6"
				echoRequestMessage.Type = ipv6.ICMPTypeEchoRequest
			}

			destAddr, err := net.ResolveIPAddr(listenNetType, dest.name)
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
						dest.protocol,
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
			dest.last = time.Now()
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

var metrics *Metrics = new(Metrics)

func main() {
	var stop bool = false
	metrics.Lock()
	metrics.startTime = time.Now()
	metrics.Unlock()

	destinations := []*Destination{
		{name: "www.reddit.com", protocol: "udp4", interval: 1500, data: []byte("TeStDaTA")},
		{name: "google-public-dns-a.google.com", protocol: "udp6", interval: 1000, data: []byte("TesTdaTa")},
		{name: "www.google.com", protocol: "udp6", interval: 750, data: []byte("TEsTDaTA")},
		{name: "www.yahoo.com", protocol: "udp4", interval: 5000, data: []byte("teSTdaTA")},
		{name: "www.amazon.com", protocol: "udp4", interval: 10000, data: []byte("tEsTdATa")},
		{name: "1.1.1.1", protocol: "udp4", interval: 2000, data: []byte("tEsTdATa")},
		{name: "::1", protocol: "udp6", interval: 500, data: []byte("tEsTdATa")},
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
				if destination.last.Add(time.Duration(destination.interval) * time.Millisecond).Before(time.Now()) {
					// Slight hack to prevent the loop from attempting to send again immediately after dispatch
					// The value will be updated again after a transmit attempt is made
					destination.last = time.Now()
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
