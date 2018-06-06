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
	"bytes"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log"
	"time"
)

const (
	IODeadline       = 2 * time.Second
	MaxPayloadSize   = 32
	MaxProbeTTL      = 30
	MinProbeInterval = 200
	MinProbeTTL      = 3
	ProtoICMP        = 1
	ProtoICMPv6      = 58
)

const (
	_ = iota
	ProtoUDP4 uint8 = iota
	ProtoUDP6
)

var DataOrder binary.ByteOrder = binary.LittleEndian

type Magic uint8
var MagicV1 Magic = 146

func (r *Magic) Decode(data []byte) error {
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, DataOrder, r); err != nil {
		log.Printf("WARN: Unable to decode Magic: %s. Value: %v\n", err, data)
		return err
	}
	return nil
}

func (r *Magic) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, DataOrder, r)
	if err != nil {
		log.Printf("ERROR: Unable to Encode Magic: %s. Value %v\n", err, r)
		return nil, err
	}

	return buf.Bytes(), nil
}

/*
 * Body - This is an echo request/reply body, which can be marshalled and
 * unmarshalled in a friendly way using the 'binary' package.
 *
 * 'Magic' - A uint8 magic value used to verify we have received the data we expect
 * in a format we understand. If 'Magic' is incorrect, processing the data should
 * stop immediately. Magic is not part of the Body struct, and must be
 * validated separately.
 *
 * 'Timestamp' - A int64 representation of a nanoseconds Unix timestamp
 *
 * 'Site' - The site ID that sent the probe request
 *
 * 'Host' - The host ID that sent the probe request
 */
type Body struct {
	Timestamp int64
	Site      uint32
	Host      uint32
}

func (r *Body) Decode(data []byte) error {
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, DataOrder, r); err != nil {
		log.Printf("ERROR: Unable to Decode Body. %s\n", err)
		return err
	}
	return nil
}

func (r *Body) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, DataOrder, r)
	if err != nil {
		log.Printf("ERROR: Unable to Encode Body. %s\n", err)
		return nil, err
	}

	return buf.Bytes(), nil
}

func ValidateMagic(data []byte) bool {
	var magic Magic

	err := magic.Decode(data)
	if err != nil {
		log.Printf("WARN: Magic byte could not be read from response packet. %s.\n", err)
		return false
	}

	switch magic {
	case MagicV1:
		return true
	default:
		return false
	}
}

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
type Source struct {
	SourceLocation uint32
	SourceHost     uint32
	SourceID       uint16
	Address        string
}

func (r *Source) String() string {
	return fmt.Sprintf("Location: %d, Host: %d, Source ID: %d, Address: %s\n",
		r.SourceLocation, r.SourceHost,r.SourceID, r.Address)
}

func (r *Source) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO sources(location, host, sourceid, address) VALUES(?, ?, ?, ?)`
	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR: beginning Source transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: preparing Source transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(r.SourceID, r.Address)
	if err != nil {
		log.Printf("ERROR: executing Source transaction. %s\n", err)
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

func GetSources(db *sql.DB) []Source {
	var sources []Source
	sqlstmnt := `SELECT location, host, sourceid, address FROM sources`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: querying sources. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		s := Source{}
		err = rows.Scan(&s.SourceLocation, &s.SourceHost, &s.SourceID, &s.Address)
		if err != nil {
			log.Printf("ERROR: querying sources. %s\n", err)
			return nil
		}
		sources = append(sources, s)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR: querying sources. %s\n", err)
		return nil
	}

	return sources
}

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
	Last     int64
	Address  string
	Interval uint32
	Timeout  uint16
	Protocol uint8
	TTL      uint8
	Active   bool
	Data     []byte
}

func (r *Destination) String() string {
	return fmt.Sprintf("Address: %s, Protocol: %d,\nInterval: %dms, Timeout: %d, TTL: %d\nData: %v\n",
		r.Address, r.Protocol, r.Interval, r.Timeout, r.TTL, r.Data)
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

func GetDestinations(db *sql.DB) []Destination {
	var destinations []Destination
	sqlstmnt := `SELECT active, address, protocol, interval, timeout, ttl, data FROM destinations`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: querying destinations. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		d := Destination{}
		err := rows.Scan(&d.Active,
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
		log.Printf("ERROR: querying destinations. %s\n", err)
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
 */
type Result struct {
	TimeStamp   int64
	Address     string
	ID          uint32
	ReceiveSite uint32
	ReceiveHost uint32
	RTT         uint32
	Type        uint16
	Code        uint16
	Sequence    uint16
	DataMatch   bool
}

func (r *Result) String() string {
	return fmt.Sprintf(
		"Timestamp: %s, Address: %s\n" +
			"Type: %d, Code: %d\n" +
			"ID: %d, Seq: %d\n" +
			"Receive Site: %d, Receive Host: %d, RTT: %d\n" +
			"DataMatch: %t\n",
		time.Unix(0, r.TimeStamp),
		r.Address,
		r.Type,
		r.Code,
		r.ID,
		r.Sequence,
		r.ReceiveSite,
		r.ReceiveHost,
		r.RTT,
		r.DataMatch)
}

func (r *Result) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO results(rtime, address, rsite, rhost, rtt, rtype, rcode, rid, rseq, datamatch)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR: beginning Result transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: preparing Result transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(r.TimeStamp, r.Address, r.Type, r.Code, r.ID, r.Sequence, r.DataMatch)
	if err != nil {
		log.Printf("ERROR: executing Result transaction. %s\n", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("ERROR: rolling back Result transaction. %s.\n", rbErr)
		}

		return err
	}
	tx.Commit()

	return nil
}

func CreateResultsTable(db *sql.DB) error {
	sqlstmnt := `CREATE TABLE IF NOT EXISTS results (
		id INTEGER NOT NULL PRIMARY KEY,
		rtime TIMESTAMP NOT NULL,
		address TEXT NOT NULL,
		rsite INTEGER NOT NULL,
		rhost INTEGER NOT NULL,
		rtt INTEGER NOT NULL,
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

func GetResults(db *sql.DB) []Result {
	var results []Result
	sqlstmnt := `SELECT rtime, address, rsite, rhost, rtt, rtype, rcode, rid, rseq, datamatch FROM results`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: querying Results. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		r := Result{}
		err = rows.Scan(&r.TimeStamp, &r.Address, &r.ReceiveSite, &r.ReceiveHost, &r.RTT,
			&r.Type, &r.Code, &r.ID, &r.Sequence, &r.DataMatch)
		if err != nil {
			log.Printf("ERROR: querying Results. %s\n", err)
			return nil
		}
		results = append(results, r)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR: querying Results. %s\n", err)
		return nil
	}

	return results
}

func BatchResultWriter(results []Result, sqldb *sql.DB) error {
	// Given a collection of Result struct, commit them as a single
	// batch in a single begin/end tran instead of as individual
	// transactions
	sqlstmnt := `INSERT INTO results(rtime, address, rsite, rhost, rtt, rtype, rcode, rid, rseq, datamatch)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	tx, err := sqldb.Begin()
	if err != nil {
		log.Printf("ERROR: beginning batch Result transaction. %s\n", err)
		return err
	}

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: preparing batch Result transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	for _, result := range results {
		_, err = stmt.Exec(result.TimeStamp,
			result.Address,
			result.Type,
			result.Code,
			result.ID,
			result.Sequence,
			result.DataMatch)

		if err != nil {
			log.Printf("ERROR: executing Result transaction. %s\n", err)
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("ERROR: rolling back Result transaction. %s.\n", rbErr)
			}

			return err
		}
	}

	tx.Commit()
	return nil
}
