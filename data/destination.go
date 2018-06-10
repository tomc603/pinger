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

package data

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

/*
 * Destinations - Database table 'destinations', used to store probe endpoints and parameters.
 * The 'active' field is used to determine whether or not a destination should be
 * probed or skipped during the probe loop.
 * TODO: Potential optimization- Select Destinations where active is true only.
 *
 * The 'address' field is not unique, and should not be collapsed into single
 * Destination instances when read from the database since parameters could be
 * different in each record.
 *
 * 'protocol' should be one of the constants Proto*, which leaves room for future types.
 *
 * An 'interval' is specified in milliseconds, and we should probably define a minimum to
 * make sure probes aren't abused.
 *
 * Currently, 'timeout' is not enforced since there's no good way to inform a listener
 * that a probe has been sent, or that a received probe should be ignored or marked as late.
 *
 * The 'ttl' field isn't enforced currently. When it is, it may be null, which means we
 * shouldn't specify a value to the probe sender. Otherwise, this specifies the hop limit
 * for a probe.
 * TODO: Enforce TTL if specified.
 *
 * 'data' is a BLOB ([]byte) field that contains the exact data to be placed into a probe's
 * payload. If the field is NULL, we shouldn't populate the payload at all.
 */
type Destination struct {
	ticker   *time.Ticker
	ID       int
	Address  string
	Interval uint32
	Timeout  uint16
	Protocol uint8
	TTL      uint8
	Active   bool
	stopFlag bool
	Data     []byte
}

func (r *Destination) Stop() {
	// Stop the Ticker, and set the stopFlag semaphore on this goroutine.
	// To stop the running goroutine, you may also close the stop channel.
	log.Printf("%s: Stop()\n", r.Address)
	r.ticker.Stop()
	r.stopFlag = true
}

func (r *Destination) Start(namech chan *Destination, stopch chan bool, wg *sync.WaitGroup) {
	r.ticker = time.NewTicker(time.Duration(r.Interval) * time.Millisecond)

	go func(namech chan *Destination, stopch chan bool, wg *sync.WaitGroup) {
		log.Printf("%s: Start()\n", r.Address)
		// We don't want to abandon any running coroutines, so handle
		// tracking them with a waitgroup inside the routine itself.
		wg.Add(1)
		defer wg.Done()

		for {
			if r.stopFlag {
				break
			}
			select {
			case <-stopch:
				r.stopFlag = true
				break
			case <-r.ticker.C:
				namech <- r
			}
		}
	}(namech, stopch, wg)
}

func (r *Destination) String() string {
	return fmt.Sprintf("ID: %d, Address: %s, Protocol: %d,\nInterval: %dms, Timeout: %d, TTL: %d\nData: %v\n",
		r.ID, r.Address, r.Protocol, r.Interval, r.Timeout, r.TTL, r.Data)
}

func (r *Destination) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO destinations(active, address, protocol, interval, timeout, ttl, data)
		VALUES(?, ?, ?, ?, ?, ?, ?)`

	// Data Validation
	if r.Protocol < ProtoUDP4 || r.Protocol > ProtoUDP6 {
		return fmt.Errorf("ERROR: destination %s protocol %d is out of bounds", r.Address, r.Protocol)
	}

	if r.Interval < MinProbeInterval {
		return fmt.Errorf("ERROR: destination %s interval %d too low", r.Address, r.Interval)
	}

	if r.TTL < MinProbeTTL {
		return fmt.Errorf("ERROR: destination %s TTL %d too small", r.Address, r.TTL)
	} else if r.TTL > MaxProbeTTL {
		return fmt.Errorf("ERROR: destination %s TTL %d too large", r.Address, r.TTL)
	}

	if len(r.Data) > MaxPayloadSize {
		return fmt.Errorf("ERROR: destination %s payload too large. Current: %d, Maximum: %d", r.Address, len(r.Data), MaxPayloadSize)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR: beginning Destination transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: preparing Destination transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(r.Active, r.Address, r.Protocol, r.Interval, r.Timeout, r.TTL, r.Data)
	if err != nil {
		log.Printf("ERROR: executing Destination transaction. %s\n", err)
		return err
	}
	tx.Commit()

	return nil
}

func CreateDestinationsTable(db *sql.DB) error {
	sqlstmnt := `CREATE TABLE IF NOT EXISTS destinations (
		id INTEGER NOT NULL PRIMARY KEY,
		active BOOL,
		address TEXT NOT NULL,
		protocol INTEGER NOT NULL,
		interval INTEGER NOT NULL,
		timeout INTEGER,
		ttl INTEGER,
		data BLOB);`

	_, err := db.Exec(sqlstmnt)
	if err != nil {
		return err
	}

	return nil
}

func GetDestinations(db *sql.DB) []*Destination {
	var destinations []*Destination
	sqlstmnt := `SELECT id, active, address, protocol, interval, timeout, ttl, data FROM destinations`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: querying destinations. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		// Booleans should initialize to false, but I'm old school and I
		// like being explicit so behavior changes never surprise me.
		d := Destination{stopFlag: false}
		err := rows.Scan(&d.ID,
			&d.Active,
			&d.Address,
			&d.Protocol,
			&d.Interval,
			&d.Timeout,
			&d.TTL,
			&d.Data)
		if err != nil {
			log.Printf("ERROR: querying destinations. %s\n", err)
			return nil
		}

		if ! d.Active {
			// Destination is marked inactive, so don't bother returning it.
			// WARNING: This could be buggy if we expect all destinations to
			// be returned, regardless of state.
			continue
		}

		// Data Validation
		// Don't allow out-of-bounds data, even if it has been inserted manually.
		if d.Protocol < ProtoUDP4 || d.Protocol > ProtoUDP6 {
			continue
		}

		if d.Interval < MinProbeInterval {
			log.Printf("WARN: ID %d: Destination %s interval too low. Using minimum %d.\n", d.ID, d.ID, d.Address, MinProbeInterval)
			d.Interval = MinProbeInterval
		}

		if d.TTL < MinProbeTTL {
			log.Printf("WARN: ID %d: Destination %s TTL %d too small. Using minimum %d.\n", d.ID, d.Address, d.TTL, MinProbeTTL)
			d.TTL = MinProbeTTL
		} else if d.TTL > MaxProbeTTL {
			log.Printf("WARN: ID %d: Destination %s TTL %d too large. Using maximum %d.\n", d.Address, d.TTL, MaxProbeTTL)
			d.TTL = MaxProbeTTL
		}

		if len(d.Data) > MaxPayloadSize {
			log.Printf("WARN: ID %d: Destination %s payload too large. Using maximum %d.\n", d.ID, d.Address, MaxPayloadSize)
			d.Data = d.Data[:MaxPayloadSize]
		}
		destinations = append(destinations, &d)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR: querying destinations. %s\n", err)
		return nil
	}

	return destinations
}
