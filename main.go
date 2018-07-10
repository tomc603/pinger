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
	"database/sql"
	"encoding/binary"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TODO: Make SenderID & SiteID config parameters. They should match values in the Senders table.
// TODO: Make DbPath a DSN, and a config parameter.
const (
	DbPath           = "/Users/tcameron/pinger.sqlite3"
	IODeadline       = 2 * time.Second
	MaxPayloadSize   = 32
	MaxProbeTTL      = 30
	MinProbeInterval = 200
	MinProbeTTL      = 3
	ProtoICMP        = 1
	ProtoICMPv6      = 58
	SenderID         = 121
	SiteID           = 101
)

const (
	_               = iota
	ProtoUDP4 uint8 = iota
	ProtoUDP6
)

var (
	// TODO: Make StatsInterval a config parameter.
	// TODO: Make ResultBatchSize a config parameter. If 0, do not batch results.
	DataOrder        binary.ByteOrder = binary.LittleEndian
	db_metrics                        = new(DbMetrics)
	DestInterval                      = 60
	MagicV1          Magic            = 146
	listener_metrics                  = new(ListenerMetrics)
	ResultBatchSize                   = 10
	sender_metrics                    = new(SenderMetrics)
	StatsInterval                     = 60
)

//destinations := []*Destination{
//	{Address: "google-public-dns-a.google.com", Protocol: ProtoUDP6, Interval: 1000, Data: []byte("TesTdaTa"), Active:true},
//	{Address: "www.yahoo.com", Protocol: ProtoUDP4, Interval: 5000, Data: []byte("teSTdaTA"), Active:true},
//	{Address: "www.amazon.com", Protocol: ProtoUDP4, Interval: 10000, Data: []byte("tEsTdATa"), Active:true},
//	{Address: "1.1.1.1", Protocol: ProtoUDP4, Interval: 2000, Data: []byte("tEsTdATa"), Active:false},
//}

func main() {
	stop := false

	destWG := sync.WaitGroup{}
	pingWG := sync.WaitGroup{}
	receiveWG := sync.WaitGroup{}
	resultWG := sync.WaitGroup{}

	namech := make(chan *Destination, 100)
	resultch := make(chan Result, 100)
	sigch := make(chan os.Signal, 5)
	stopch := make(chan bool)

	signal.Notify(sigch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGUSR1)

	statsTicker := &time.Ticker{}
	if StatsInterval > 0 {
		statsTicker = time.NewTicker(time.Duration(StatsInterval) * time.Second)
	}

	sqldb, err := sql.Open("sqlite3", DbPath)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
	}
	defer sqldb.Close()

	// Create tables if they don't exist.
	if err := CreateSourcesTable(sqldb); err != nil {
		log.Fatalf("ERROR: Sources table could not be created. %s.\n", err)
	}
	if err := CreateDestinationsTable(sqldb); err != nil {
		log.Fatalf("ERROR: Destinations table could not be created. %s.\n", err)
	}
	if err := CreateResultsTable(sqldb); err != nil {
		log.Fatalf("ERROR: Results table could not be created. %s.\n", err)
	}

	destinations := GetDestinations(sqldb)

	listener_metrics.Lock()
	listener_metrics.startTime = time.Now()
	listener_metrics.Unlock()

	go resultWriter(resultch, sqldb, &resultWG)
	go v6Listener(stopch, resultch, &receiveWG)
	go v4Listener(stopch, resultch, &receiveWG)

	sender_metrics.Lock()
	sender_metrics.startTime = time.Now()
	sender_metrics.Unlock()

	go ping(namech, stopch, &pingWG)
	go watchDestinations(sqldb, destinations, stopch, &destWG)

	for _, destination := range destinations {
		destination.Start(namech, stopch, &destWG)
	}

	for {
		if stop {
			for _, destination := range destinations {
				destination.Stop()
			}

			break
		}

		select {
		case <-statsTicker.C:
			log.Printf("%s\n", listener_metrics)
			log.Printf("%s\n", sender_metrics)
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
				log.Printf("%s\n", listener_metrics)
				log.Printf("%s\n", sender_metrics)
			}
			break
		}
	}

	close(stopch)
	receiveWG.Wait()

	// Wait until all of the receivers have stopped and returned,
	// then stop the result processor. There shouldn't be a race here,
	// but I'm being cautious nonetheless.
	close(resultch)
	resultWG.Wait()

	close(namech)
	destWG.Wait()
	pingWG.Wait()

	log.Println("Exiting main function.")
}
