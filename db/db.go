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

package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	MaxPayloadSize   = 32
	MaxProbeTTL      = 30
	MinProbeInterval = 200
	MinProbeTTL      = 3
)

const (
	_ = iota
	ProtoUDP4 int = iota
	ProtoUDP6
)

/*
 * Sources - Database table 'sources', used for managing probe source instances.
 * The SourceID is used to populate the upper 8 bits of an ICMP Message's Identifier
 * field. SourceIDs may be duplicated in the database, but must be unique for each
 * SourceLocation. SourceLocation and SourceHost are both unique fields.
 * TODO: Investigate storing addresses as BLOB ([]byte) instead of TEXT (string)
 *
 * Fields:
 *   location - integer
 *   host     - integer
 *   sourceid - integer
 *   address  - string
 */
type DbSource struct {
	SourceLocation int
	SourceHost int
	SourceID int
	Address  string
}

func (ds *DbSource) String() string {
	return fmt.Sprintf("Source ID: %d - Address: %s\n", ds.SourceID, ds.Address)
}

func (ds *DbSource) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO sources(location, host, sourceid, address) values(?, ?, ?, ?)`
	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR beginning Source transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR preparing Source transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(ds.SourceID, ds.Address)
	if err != nil {
		log.Printf("ERROR executing Source transaction. %s\n", err)
		return err
	}
	tx.Commit()

	return nil
}

func CreateSourcesTable(db *sql.DB) error {
	sqlstmnt := `CREATE TABLE IF NOT EXISTS sources (
		id INTEGER NOT NULL PRIMARY KEY,
		location INTEGER NOT NULL UNIQUE,
		host INTEGER NOT NULL UNIQUE,
		sourceid INTEGER NOT NULL,
		address TEXT NOT NULL);`

	_, err := db.Exec(sqlstmnt)
	if err != nil {
		return err
	}

	return nil
}

func GetSources(db *sql.DB) []DbSource {
	var sources []DbSource
	sqlstmnt := `SELECT location, host, sourceid, address FROM sources`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR querying sources. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		s := DbSource{}
		err = rows.Scan(&s.SourceLocation, &s.SourceHost, &s.SourceID, &s.Address)
		if err != nil {
			log.Printf("ERROR querying sources. %s\n", err)
			return nil
		}
		sources = append(sources, s)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR querying sources. %s\n", err)
		return nil
	}

	return sources
}

/*
 * Destinations - Database table 'destinations', used to store probe endpoints and parameters.
 * The 'active' field is used to determine whether or not a destination should be
 * probed or skipped during the probe loop.
 * TODO: Potential memory optimization- Remove DbDestination from slice if active is false.
 *
 * The 'address' field is not unique, and should not be collapsed into single
 * DbDestination instances when read from the database since parameters could be
 * different in each record.
 *
 * 'protocol' should be one of the constants Proto*, which leaves room for future types.
 *
 * An 'interval' is specified in milliseconds, and we should probably define a minimum to
 * make sure probes aren't abused.
 * TODO: Limit to a safe minimum on SELECT.
 *
 * Currently, 'timeout' is not enforced since there's no good way to inform a listener
 * that a probe has been sent, or that a received probe should be ignored or marked as late.
 *
 * The 'ttl' field isn't enforced currently. When it is, it may be null, which means we
 * shouldn't specify a value to the probe sender. Otherwise, this specifies the hop limit
 * for a probe.
 * TODO: Enforce TTL if specified, limit to a sane maximum and minimum on SELECT.
 *
 * 'data' is a BLOB ([]byte) field that contains the exact data to be placed into a probe's
 * payload. If the field is NULL, we shouldn't populate the payload at all.
 * TODO: Limit the maximum size of 'data', trim extra bytes on SELECT.
 */
type DbDestination struct {
	Active   bool
	Address  string
	Protocol int
	Interval int
	Timeout  int
	TTL      int
	Data     []byte
}

func (dd *DbDestination) String() string {
	return fmt.Sprintf("Address: %s, Protocol: %d,\nInterval: %dms, Timeout: %d, TTL: %d\nData: %v\n",
		dd.Address, dd.Protocol, dd.Interval, dd.Timeout, dd.TTL, dd.Data)
}

func (dd *DbDestination) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO destinations(active, address, protocol, interval, timeout, ttl, data)
		VALUES(?, ?, ?, ?, ?, ?, ?)`

	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR beginning Source transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR preparing Source transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(dd.Active, dd.Address, dd.Protocol, dd.Interval, dd.Timeout, dd.TTL, dd.Data)
	if err != nil {
		log.Printf("ERROR executing Source transaction. %s\n", err)
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

func GetDestinations(db *sql.DB) []DbDestination {
	var destinations []DbDestination
	sqlstmnt := `SELECT active, address, protocol, interval, timeout, ttl, data FROM destinations`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR querying destinations. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		d := DbDestination{}
		err = rows.Scan(&d.Active,
			&d.Address,
			&d.Protocol,
			&d.Interval,
			&d.Timeout,
			&d.TTL,
			&d.Data)
		if err != nil {
			log.Printf("ERROR querying destinations. %s\n", err)
			return nil
		}

		// If Protocol is invalid, don't add 'd' to 'destinations'.
		if d.Protocol < ProtoUDP4 || d.Protocol > ProtoUDP6 {
			continue
		}

		if d.Interval < MinProbeInterval {
			log.Printf("WARN: Destination %s interval too low. Using minimum %d.\n", d.Address, MinProbeInterval)
			d.Interval = MinProbeInterval
		}

		if d.TTL < MinProbeTTL {
			log.Printf("WARN: Destination %s TTL too small. Using minimum %d.\n", d.Address, MinProbeInterval)
			d.TTL = MinProbeTTL
		} else if d.TTL > MaxProbeTTL {
			log.Printf("WARN: Destination %s TTL too large. Using maximum %d.\n", d.Address, MaxProbeTTL)
			d.TTL = MaxProbeTTL
		}

		if len(d.Data) > MaxPayloadSize {
			log.Printf("WARN: Destination %s payload too large. Using maximum %d.\n", d.Address, MaxPayloadSize)
			d.Data = d.Data[:MaxPayloadSize]
		}
		destinations = append(destinations, d)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR querying destinations. %s\n", err)
		return nil
	}

	return destinations
}


/*
 * Results - Database table 'results' contains responses to probes.
 *
 * 'rtime' is time.Now() at the receiving host when the packet was decoded.
 *
 * 'address' will be the IP Address of the host responding to the probe, and should be
 * indexed for better search performance. This field could also be a BLOB ([]byte)
 * instead of a string, although there may be performance implications with this field type.
 *
 * 'rtype' and 'rcode' are ICMP control message values that indicate success or various
 * failure types.
 *
 * 'rid' contains the SenderID of the original sender host encoded in its upper 8 bits.
 * combinding 'rid' and 'rseq' allows sent probe correlation with received probes. This
 * will be important in a future version when TTL and RTT limits are added. For now, it
 * just serves as a way to correlate sent and received probe information during display.
 *
 * 'datamatch' requires that the Destination be checked for this received message, and
 * the data field in the DB be compared with the data received in this message after
 * the prepended metadata is stripped.
 *
 * TODO: Add receiving site, host ID, and RTT
 */
type DbResult struct {
	TimeStamp time.Time
	Address   string
	Type      int
	Code      int
	ID        int
	Sequence  int
	DataMatch bool
}

func (dr *DbResult) String() string {
	return fmt.Sprintf("Address: %s\nType: %s, Code: %d\nID: %d, Seq: %d\nDataMatch: %t\n",
		dr.Address, dr.Type, dr.Code, dr.ID, dr.Sequence, dr.DataMatch)
}

func (dr *DbResult) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO results(rtime, address, rtype, rcode, rid, rseq, datamatch)
		VALUES(?, ?, ?, ?, ?, ?, ?)`

	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR beginning Source transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR preparing Source transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(dr.TimeStamp, dr.Address, dr.Type, dr.Code, dr.ID, dr.Sequence, dr.DataMatch)
	if err != nil {
		log.Printf("ERROR executing Source transaction. %s\n", err)
		return err
	}
	tx.Commit()

	return nil
}

func CreateResultsTable(db *sql.DB) error {
	sqlstmnt := `CREATE TABLE IF NOT EXISTS results (
		id INTEGER NOT NULL PRIMARY KEY,
		address TEXT NOT NULL,
		rtime TIMESTAMP NOT NULL,
		rtype INTEGER NOT NULL,
		rcode INTEGER NOT NULL,
		rid INTEGER NOT NULL,
		rseq INTEGER NOT NULL,
		datamatch BOOL);`

	_, err := db.Exec(sqlstmnt)
	if err != nil {
		return err
	}

	return nil
}

func GetResults(db *sql.DB) []DbResult {
	var results []DbResult
	sqlstmnt := `SELECT address, rtime, rtype, rcode, rid, rseq, datamatch FROM results`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR querying results. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		r := DbResult{}
		err = rows.Scan(&r.Address, &r.TimeStamp, &r.Type,
			&r.Code, &r.ID, &r.Sequence, &r.DataMatch)
		if err != nil {
			log.Printf("ERROR querying results. %s\n", err)
			return nil
		}
		results = append(results, r)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR querying results. %s\n", err)
		return nil
	}

	return results
}
