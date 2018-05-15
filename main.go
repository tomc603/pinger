package main

import (
	"log"
	"net"
	"os"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

const (
	ProtoICMP = 1
	ProtoICMPv6 = 58
)

func main() {
	c, err := icmp.ListenPacket("udp6", "::")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	wmsg := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff,
			Seq: 1,
			Data: []byte("ICMPTeST"),
		},
	}

	mmsg, err := wmsg.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := c.WriteTo(mmsg, &net.UDPAddr{IP: net.ParseIP("2001:4860:4860::8888")}); err != nil {
		log.Fatal(err)
	}

	for {
		rbuf := make([]byte, 1500)
		n, peer, err := c.ReadFrom(rbuf)
		if err != nil {
			log.Fatal(err)
		}

		rmsg, err := icmp.ParseMessage(ProtoICMPv6, rbuf[:n])
		if err != nil {
			log.Fatal(err)
		}

		switch rmsg.Type {
		case ipv6.ICMPTypeEchoReply:
			// TODO: Convert the message into an icmp.Echo to parse out the data.
			//recho := rmsg.Body.(icmp.Echo)
			log.Printf("Reflection from %v\n %#v", peer, rmsg)
		case ipv6.ICMPTypeEchoRequest:
			continue
		default:
			log.Printf("Got %v from %v.\n%#v", rmsg.Type, peer, rmsg)
		}
	}
}
