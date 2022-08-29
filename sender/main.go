package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tomc603/pinger/data"
)

// TODO: Read SenderID, SiteID, and DbPath from the config file, environment, or command line.
//
//	They should match values in the Senders table.
//
// TODO: Make DbPath a DSN.
const (
	SenderID = 121                              // Read this value from the config file, environment, or command line
	SiteID   = 101                              // Read this value from the config file, environment, or command line
	DbPath   = "/Users/tcameron/pinger.sqlite3" // Read this value from the config file, environment, or command line
)

// TODO: Read StatsInterval from the config file, environment, or command line.
var (
	StatsInterval = 60
	DestInterval  = 60
	metrics       = new(Metrics)
)

// TODO: Add functions for other types of probe than ICMP.
func main() {
	var stop = false

	destWG := sync.WaitGroup{}
	pingWG := sync.WaitGroup{}

	sigch := make(chan os.Signal, 5)
	namech := make(chan *data.Destination, 100)
	stopch := make(chan bool)

	signal.Notify(sigch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGUSR1)

	metrics.Lock()
	metrics.startTime = time.Now()
	metrics.Unlock()

	sqldb, err := sql.Open("sqlite3", DbPath)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
	}
	defer sqldb.Close()

	// Create tables if they don't exist.
	if err := data.CreateSourcesTable(sqldb); err != nil {
		log.Fatalf("ERROR: Sources table could not be created. %s.\n", err)
	}
	if err := data.CreateDestinationsTable(sqldb); err != nil {
		log.Fatalf("ERROR: Destinations table could not be created. %s.\n", err)
	}
	if err := data.CreateResultsTable(sqldb); err != nil {
		log.Fatalf("ERROR: Results table could not be created. %s.\n", err)
	}

	statsTicker := &time.Ticker{}
	if StatsInterval > 0 {
		statsTicker = time.NewTicker(time.Duration(StatsInterval) * time.Second)
	}

	go ping(namech, stopch, &pingWG)

	//destinations := []*data.Destination{
	//	{Address: "google-public-dns-a.google.com", Protocol: data.ProtoUDP6, Interval: 1000, Data: []byte("TesTdaTa"), Active:true},
	//	{Address: "www.yahoo.com", Protocol: data.ProtoUDP4, Interval: 5000, Data: []byte("teSTdaTA"), Active:true},
	//	{Address: "www.amazon.com", Protocol: data.ProtoUDP4, Interval: 10000, Data: []byte("tEsTdATa"), Active:true},
	//	{Address: "1.1.1.1", Protocol: data.ProtoUDP4, Interval: 2000, Data: []byte("tEsTdATa"), Active:false},
	//}
	destinations := data.GetDestinations(sqldb)
	go watchDestinations(sqldb, destinations, stopch, &destWG)

	for _, destination := range destinations {
		destination.Start(namech, stopch, &destWG)
	}

	for {
		if stop {
			for _, destination := range destinations {
				destination.Stop()
			}
			statsTicker.Stop()
			break
		}

		select {
		case <-statsTicker.C:
			log.Printf("%s\n", metrics)
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
				log.Printf("%s\n", metrics)
			}
			break
		}
	}

	// Tell the destination routine(s) that we're finished, and wait for a graceful stop.
	close(stopch)
	close(namech)
	destWG.Wait()
	pingWG.Wait()
	log.Printf("Exiting ping sender.")
}
