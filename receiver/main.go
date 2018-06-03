package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tomc603/pinger/sql"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ProtoICMP   = 1
	ProtoICMPv6 = 58
	SenderID    = 121
	IODeadline = 2 * time.Second
)

type Metrics struct {
	sync.RWMutex
	v4Sent          uint
	v4ReceiveFailed uint
	v4ParseFailed   uint
	v4Bytes         uint
	v6Sent          uint
	v6ReceiveFailed uint
	v6ParseFailed   uint
	v6Bytes         uint
	startTime     time.Time
}

func (m *Metrics) Addv4Received(i uint) {
	m.Lock()
	m.v4Sent += i
	m.Unlock()
}

func (m *Metrics) Addv4ReceiveFailed(i uint) {
	m.Lock()
	m.v4ReceiveFailed += i
	m.Unlock()
}

func (m *Metrics) Addv4ParseFailed(i uint) {
	m.Lock()
	m.v4ParseFailed += i
	m.Unlock()
}

func (m *Metrics) Addv4Bytes(i uint) {
	m.Lock()
	m.v4Bytes += i
	m.Unlock()
}

func (m *Metrics) Addv6Received(i uint) {
	m.Lock()
	m.v6Sent += i
	m.Unlock()
}

func (m *Metrics) Addv6ReceiveFailed(i uint) {
	m.Lock()
	m.v6ReceiveFailed += i
	m.Unlock()
}

func (m *Metrics) Addv6ParseFailed(i uint) {
	m.Lock()
	m.v6ParseFailed += i
	m.Unlock()
}

func (m *Metrics) Addv6Bytes(i uint) {
	m.Lock()
	m.v6Bytes += i
	m.Unlock()
}

func (m *Metrics) String() string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("Uptime: %v\n" +
		"IPv4 received: %d\n" +
		"IPv4 receive error: %d\n" +
		"IPv4 parse error: %d\n" +
		"IPv4 bytes: %d\n" +
		"IPv6 received: %d\n" +
		"IPv6 receive error: %d\n" +
		"IPv6 parse error: %d\n" +
		"IPv6 bytes: %d\n" +
		"Total received: %d\n" +
		"Total receive error: %d\n" +
		"Total parse error: %d\n" +
		"Total bytes: %d\n",
		time.Since(m.startTime),
		m.v4Sent, m.v4ReceiveFailed, m.v4ParseFailed, m.v4Bytes,
		m.v6Sent, m.v6ReceiveFailed, m.v6ParseFailed, m.v6Bytes,
		m.v4Sent + m.v6Sent,
		m.v4ReceiveFailed + m.v6ReceiveFailed,
		m.v4ParseFailed + m.v6ParseFailed,
		m.v4Bytes + m.v6Bytes)
}

func v4Listener(stopch chan bool, wg *sync.WaitGroup) {
	var stop bool = false
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

			switch receiveMessage.Type {
			case ipv4.ICMPTypeEchoReply:
				metrics.Addv4Received(1)
				metrics.Addv4Bytes(uint(n))

				// TODO: Decode the message, compare the data payload and record the receipt
				// TODO: Decode probe sending location, host, and time from the message payload.
				echoReply := receiveMessage.Body.(*icmp.Echo)
				log.Printf("ID %d, Seq: %d, From: %s\n", echoReply.ID, echoReply.Seq, peer)
			}
		}
	}
	log.Println("Ping v4Listener stopped.")
}

func v6Listener(stopch chan bool, wg *sync.WaitGroup) {
	var stop bool = false
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

			switch receiveMessage.Type {
			case ipv6.ICMPTypeEchoReply:
				metrics.Addv6Received(1)
				metrics.Addv6Bytes(uint(n))

				// TODO: Decode the message, compare the data payload and record the receipt
				// TODO: Decode probe sending location, host, and time from the message payload.
				echoReply := receiveMessage.Body.(*icmp.Echo)
				log.Printf("ID %d, Seq: %d, From: %s\n", echoReply.ID, echoReply.Seq, peer)
			}
		}
	}
	log.Println("Ping v6Listener stopped.")
}

var metrics *Metrics = new(Metrics)

func main() {
	var stop bool = false
	receiveWG := sync.WaitGroup{}
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

	db, err := sql.Open("sqlite3", "./db.sqlite3")
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
	}
	defer db.Close()

	receiveWG.Add(2)
	go v6Listener(stopch, &receiveWG)
	go v4Listener(stopch, &receiveWG)

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

	close(stopch)
	receiveWG.Wait()
	log.Printf("Exiting ping receiver.")
}
