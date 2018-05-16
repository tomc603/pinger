package main

import (
	"bytes"
	"log"
	"net"
	"os"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ProtoICMP = 1
	ProtoICMPv6 = 58
)

func ping(conn *icmp.PacketConn, addr string) {
	echoRequestBody := icmp.Echo{
		ID: os.Getpid() & 0xffff,
		Seq: 1,
		Data: []byte("ICMPTeST"),
	}

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

		if _, err := conn.WriteTo(echoRequest, &net.UDPAddr{IP: v6Addr.IP}); err != nil {
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
			log.Printf("No v6 Address: '%s'. %s", err.(*net.AddrError).Addr, err.(*net.AddrError).Err)
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

		if _, err := conn.WriteTo(echoRequest, &net.UDPAddr{IP: v4Addr.IP}); err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ping(conn,"www.reddit.com.")

	//destinationV6Addr, err := net.ResolveIPAddr("udp6", "google-public-dns-a.google.com.")
	//if err != nil {
	//	log.Fatal(err)
	//}

	//if _, err := conn.WriteTo(echoRequest, &net.UDPAddr{IP: destinationV6Addr.IP}); err != nil {
	//	log.Fatal(err)
	//}

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
			// TODO: Convert the message into an icmp.Echo to parse out the data.
			echoReply := receiveMessage.Body.(*icmp.Echo)
			log.Printf("ID %v, Seq: %v, From: %v\nData: %v\n",
				echoReply.ID,
				echoReply.Seq,
				peer,
				bytes.Compare(echoReply.Data, echoRequestBody.Data) == 0)
		case ipv6.ICMPTypeEchoRequest:
			continue
		default:
			log.Printf("Got %v from %v.\n%#v", receiveMessage.Type, peer, receiveMessage)
		}
	}
}
