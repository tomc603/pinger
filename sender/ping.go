/*
 *    Copyright 2018 Tom Cameron
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 *
 */

package main

import (
	"bytes"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tomc603/pinger/data"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

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

			bodyBuffer := new(bytes.Buffer)
			magicData, magicErr := data.MagicV1.Encode()
			if magicErr != nil {
				log.Printf("WARN: Could not encode Magic value. %s\n", magicErr)
			}
			bodyBuffer.Write(magicData)

			body := data.Body{
				Timestamp: time.Now().UnixNano(),
				Site:      SiteID,
				Host:      SenderID,
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

			echoRequest, err := echoRequestMessage.Marshal(nil)
			if err != nil {
				log.Fatal(err)
			}

			b, err := conn.WriteTo(echoRequest, &net.UDPAddr{IP: destAddr.IP})
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
