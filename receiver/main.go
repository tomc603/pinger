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

var (
	// TODO: Make StatsInterval a config parameter.
	// TODO: Make ResultBatchSize a config parameter. If 0, do not batch results.
	StatsInterval   = 60
	ResultBatchSize = 10
	metrics         = new(Metrics)
)

// TODO: Make DbPath a DSN, and a config parameter.
const DbPath = "/Users/tcameron/pinger.sqlite3"

func main() {
	var stop = false

	receiveWG := sync.WaitGroup{}
	resultWG := sync.WaitGroup{}

	resultch := make(chan data.Result, 100)
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

	// sources := db.GetSources(sqldb)

	statsTicker := &time.Ticker{}
	if StatsInterval > 0 {
		statsTicker = time.NewTicker(time.Duration(StatsInterval) * time.Second)
	}

	go resultWriter(resultch, sqldb, &resultWG)
	go v6Listener(stopch, resultch, &receiveWG)
	go v4Listener(stopch, resultch, &receiveWG)

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

	// Tell the receiver functions to stop, and wait for them.
	close(stopch)
	receiveWG.Wait()

	// Wait until all of the receivers have stopped and returned,
	// then stop the result processor. There shouldn't be a race here,
	// but I'm being cautious nonetheless.
	close(resultch)
	resultWG.Wait()

	log.Printf("Exiting ping receiver.")
}
