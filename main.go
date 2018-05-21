package main

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type destination struct {
	name string
	network string
	data []byte
}

const (
	ProtoICMP = 1
	ProtoICMPv6 = 58
	SenderID = 121
)

func ping(destinations chan destination, wg *sync.WaitGroup) {
	defer wg.Done()

	for dest := range destinations {
		listenAddress := "0.0.0.0"
		listenNetType := "ip4"
		echoRequestBody := icmp.Echo{
			ID: SenderID << 8 | 0x0000 + 1,
			Seq: 1,
			Data: []byte("ICMPTeST"),
		}
		echoRequestMessage := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &echoRequestBody,
		}

		if strings.HasSuffix(dest.network, "6") {
			listenAddress = "::"
			listenNetType = "ip6"
			echoRequestMessage.Type = ipv6.ICMPTypeEchoRequest
		}

		conn, err := icmp.ListenPacket(dest.network, listenAddress)
		if err != nil {
			log.Fatal(err)
		}

		destAddr, err := net.ResolveIPAddr(listenNetType, dest.name)
		if err != nil {
			switch err.(type) {
			case *net.DNSError:
				e := err.(*net.DNSError)
				if e.IsTimeout {
					log.Fatalf("DNS Timeout: %#v", e.Name)
				} else if e.IsTemporary {
					log.Fatalf("DNS Temporary Failure: %#v", e)
				} else {
					log.Fatalf("DNS Error: %#v", e)
				}
			case *net.AddrError:
				log.Printf("No %s Address: '%s'. %s",
					dest.network,
					err.(*net.AddrError).Addr, err.(*net.AddrError).Err)
			default:
				log.Fatalf("Unexpected error: %#v", err)
			}
		} else {
			log.Printf("Address: %v", destAddr)
			echoRequest, err := echoRequestMessage.Marshal(nil)
			if err != nil {
				log.Fatal(err)
			}

			if _, err := conn.WriteTo(echoRequest, &net.UDPAddr{IP: destAddr.IP}); err != nil {
				log.Fatal(err)
			}
		}
	}
	log.Println("Name channel closed.")
}

func v4Listener(wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := icmp.ListenPacket("udp4", "::")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	log.Println("v4Listener started.")

	for {
		receiveBuffer := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(receiveBuffer)
		if err != nil {
			log.Fatal(err)
		}

		receiveMessage, err := icmp.ParseMessage(ProtoICMP, receiveBuffer[:n])
		if err != nil {
			log.Fatal(err)
		}

		switch receiveMessage.Type {
		case ipv4.ICMPTypeEchoReply:
			echoReply := receiveMessage.Body.(*icmp.Echo)
			log.Printf("ID %v, Seq: %v, From: %v, Data: %v\n",
				echoReply.ID,
				echoReply.Seq,
				peer,
				echoReply.Data)
		}
	}
}

func v6Listener(wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := icmp.ListenPacket("udp6", "::")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	log.Println("v6Listener started.")

	for {
		receiveBuffer := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(receiveBuffer)
		if err != nil {
			log.Fatal(err)
		}

		receiveMessage, err := icmp.ParseMessage(ProtoICMPv6, receiveBuffer[:n])
		if err != nil {
			log.Fatal(err)
		}

		switch receiveMessage.Type {
		case ipv6.ICMPTypeEchoReply:
			echoReply := receiveMessage.Body.(*icmp.Echo)
			log.Printf("ID %v, Seq: %v, From: %v, Data: %v\n",
				echoReply.ID,
				echoReply.Seq,
				peer,
				echoReply.Data)
		}
	}
}
func main() {
	names := []destination{
		{name: "www.reddit.com", network: "udp4", data: []byte("TeStDaTA")},
		{name: "google-public-dns-a.google.com", network: "udp6", data: []byte("TesTdaTa")},
		{name: "www.google.com", network: "udp6", data: []byte("TEsTDaTA")},
		{name: "www.yahoo.com", network: "udp4", data: []byte("teSTdaTA")},
		{name: "www.amazon.com", network: "udp4", data: []byte("tEsTdATa")},
	}
	namech := make(chan destination, 100)

	pingWG := sync.WaitGroup{}
	receiveWG := sync.WaitGroup{}

	receiveWG.Add(2)
	go v6Listener(&receiveWG)
	go v4Listener(&receiveWG)

	pingWG.Add(1)
	go ping(namech, &pingWG)

	for _, name := range names {
		namech <- name
		time.Sleep(500 * time.Millisecond)
	}

	close(namech)
	pingWG.Wait()
	receiveWG.Wait()
}
