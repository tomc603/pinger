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
	"database/sql"
	"log"
	"sync"
	"time"
)

func watchDestinations(db *sql.DB, destinations []*Destination, stopch chan bool, wg *sync.WaitGroup) {
	stop := false
	t := time.NewTicker(time.Duration(DestInterval) * time.Second)

	wg.Add(1)
	defer wg.Done()

	log.Println("watchDestinations started.")
	for {
		if stop {
			break
		}

		select {
		case <-stopch:
			stop = true
			break
		case <-t.C:
			// On every tick, query the database for Destinations. Compare what we have
			// to what we received, and make any updates necessary. Remember to Stop()
			// any Destination that needs to be modified.

			// TODO: Decide how to track Destination objects.
			// Should we make a map, keyed on destination.Address with value pointer to
			// a Destination object? Does iterating through two lists constantly make more
			// sense? What is the performance impact once the collection gets up to tens of
			// thousands of objects?
			//
			// TODO: Add locking to Destination objects? Lock where Destination are tracked?
			// TODO: This is easier if I just atomically changed what "destinations" points to.
			// TODO: Stop deleted/inactive Destinations, Stop/Start modified Interval Destinations.
			db_metrics.AddDestinationReads(1)
			newDestinations := GetDestinations(db)
			for _, newDestination := range newDestinations {
				var foundDestination *Destination
				found := false
				modified := false

				for _, destination := range destinations {
					if newDestination.Id == destination.Id {
						found = true
						foundDestination = destination
						break
					}
				}

				if !found {
					// A new Destination was found. Append it to the slice.
					// TODO: We need to Start() the new Destination, which requires channels
					log.Printf("INFO: New Destination: %s\n", newDestination.Address)
					destinations = append(destinations, newDestination)
					db_metrics.AddAddedDestinations(1)
				} else {
					if foundDestination.Address != newDestination.Address {
						log.Printf("INFO: Updating Id %d Address: %s\n", newDestination.Id, newDestination.Address)
						modified = true
						foundDestination.Address = newDestination.Address
					}
					if foundDestination.Interval != newDestination.Interval {
						log.Printf("INFO: Updating Id %d Interval: %d\n", newDestination.Id, newDestination.Interval)
						modified = true
						foundDestination.Interval = newDestination.Interval
					}
					if foundDestination.TTL != newDestination.TTL {
						log.Printf("INFO: Updating Id %d TTL: %d\n", newDestination.Id, newDestination.TTL)
						modified = true
						foundDestination.TTL = newDestination.TTL
					}
					if foundDestination.Timeout != newDestination.Timeout {
						log.Printf("INFO: Updating Id %d Timeout: %d\n", newDestination.Id, newDestination.Timeout)
						modified = true
						foundDestination.Timeout = newDestination.Timeout
					}
					if foundDestination.Protocol != newDestination.Protocol {
						log.Printf("INFO: Updating Id %d Protocol: %d\n", newDestination.Id, newDestination.Protocol)
						modified = true
						foundDestination.Protocol = newDestination.Protocol
					}
					if foundDestination.Active != newDestination.Active {
						log.Printf("INFO: Updating Id %d Active: %t\n", newDestination.Id, newDestination.Active)
						modified = true
						foundDestination.Active = newDestination.Active
						if !foundDestination.Active {
							// TODO: Delete inactive items from the destinations slice
							foundDestination.Stop()
						}
					}
					if !bytes.Equal(foundDestination.Data, newDestination.Data) {
						log.Printf("INFO: Updating Id %d Payload\n", newDestination.Id)
						modified = true
						foundDestination.Data = newDestination.Data
					}

					if modified {
						db_metrics.AddModifiedDestinationReads(1)
					}
				}
			}

			// Check for deleted items by basically repeating the above, but reverse the slices
		DeleteCheck:
			for pos, destination := range destinations {
				found := false

				for _, newDestination := range newDestinations {
					if newDestination.Id == destination.Id {
						found = true
						break
					}
				}

				if !found {
					log.Printf("INFO: Deleting Id %d: %s\n", destination.Id, destination.Address)
					db_metrics.AddRemovedDestinations(1)
					destination.Stop()
					destinations = append(destinations[:pos], destinations[pos+1:]...)
					// We have to restart the check every time we delete and item,
					// because we're using a slice. Using a map would be SO much better.
					goto DeleteCheck
				}
			}
		}
	}
	log.Println("watchDestinations stopped.")
}
