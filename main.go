package main

import (
	"log"
	"net"
	"os"
	"sync"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ProtoICMP = 1
	ProtoICMPv6 = 58
)

func ping(names chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	v6Sender, err := icmp.ListenPacket("udp6", "::")
	if err != nil {
		log.Fatal(err)
	}
	v4Sender, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Fatal(err)
	}

	defer v6Sender.Close()
	defer v4Sender.Close()
	log.Println("ping started.")

	echoRequestBody := icmp.Echo{
		ID: os.Getpid() & 0xffff,
		Seq: 1,
		Data: []byte("ICMPTeST"),
	}

	for addr := range names {
		v6Addr, err := net.ResolveIPAddr("ip6", addr)
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
				log.Printf("No v6 Address: '%s'. %s", err.(*net.AddrError).Addr, err.(*net.AddrError).Err)
			default:
				log.Fatalf("Unexpected error: %#v", err)
			}
		} else {
			log.Printf("v6 Address: %v", v6Addr)
			echoRequestMessage := icmp.Message{
				Type: ipv6.ICMPTypeEchoRequest,
				Code: 0,
				Body: &echoRequestBody,
			}

			echoRequest, err := echoRequestMessage.Marshal(nil)
			if err != nil {
				log.Fatal(err)
			}

			if _, err := v6Sender.WriteTo(echoRequest, &net.UDPAddr{IP: v6Addr.IP}); err != nil {
				log.Fatal(err)
			}
		}

		v4Addr, err := net.ResolveIPAddr("ip4", addr)
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
				log.Printf("No v4 Address: '%s'. %s", err.(*net.AddrError).Addr, err.(*net.AddrError).Err)
			default:
				log.Fatalf("Unexpected error: %#v", err)
			}
		} else {
			log.Printf("v4 Address: %v", v4Addr)
			echoRequestMessage := icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &echoRequestBody,
			}

			echoRequest, err := echoRequestMessage.Marshal(nil)
			if err != nil {
				log.Fatal(err)
			}

			if _, err := v4Sender.WriteTo(echoRequest, &net.UDPAddr{IP: v4Addr.IP}); err != nil {
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
	names := []string{
		"www.reddit.com",
		"google-public-dns-a.google.com",
		"www.google.com",
		"www.yahoo.com",
		"www.amazon.com",
	}

	namech := make(chan string, 100)

	pingWG := sync.WaitGroup{}
	receiveWG := sync.WaitGroup{}

	receiveWG.Add(2)
	go v6Listener(&receiveWG)
	go v4Listener(&receiveWG)

	pingWG.Add(1)
	go ping(namech, &pingWG)

	for _, name := range names {
		namech <- name
	}

	close(namech)
	pingWG.Wait()
	receiveWG.Wait()
}
