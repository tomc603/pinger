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
	"log"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/tomc603/pinger/data"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

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
				Address:   peer.String(),
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
					err := echoBody.Decode(echoReply.Data[unsafe.Sizeof(data.MagicV1) : unsafe.Sizeof(echoBody)+1])
					if err == nil {
						result.ReceiveSite = echoBody.Site
						result.ReceiveHost = echoBody.Host
						result.RTT = uint32(time.Unix(0, result.TimeStamp).Sub(time.Unix(0, echoBody.Timestamp)) / time.Millisecond)
					}
				}

				result.RequestID = uint16(echoReply.ID)
				result.Sequence = uint16(echoReply.Seq)
				result.Code = uint16(receiveMessage.Code)
				result.Type = uint16(receiveMessage.Type.Protocol())

				resultchan <- result
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
				Address:   peer.String(),
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
					err := echoBody.Decode(echoReply.Data[unsafe.Sizeof(data.MagicV1) : unsafe.Sizeof(echoBody)+1])
					if err == nil {
						result.ReceiveSite = echoBody.Site
						result.ReceiveHost = echoBody.Host
						result.RTT = uint32(time.Unix(0, result.TimeStamp).Sub(time.Unix(0, echoBody.Timestamp)) / time.Millisecond)
					}
				}

				result.RequestID = uint16(echoReply.ID)
				result.Sequence = uint16(echoReply.Seq)
				result.Code = uint16(receiveMessage.Code)
				result.Type = uint16(receiveMessage.Type.Protocol())

				resultchan <- result
			}
		}
	}
	log.Println("Ping v6Listener stopped.")
}
