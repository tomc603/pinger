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
	"time"
)

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
	Id          int
	ReceiveSite uint32
	ReceiveHost uint32
	RTT         uint32
	Type        uint16
	Code        uint16
	RequestID   uint16
	Sequence    uint16
	DataMatch   bool
}

func (r *Result) Batch(tx *sql.Tx) error {
	sqlstmnt := `INSERT INTO results(rtime, address, rsite, rhost, rtt, rtype, rcode, rid, rseq, datamatch)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: preparing Result transaction. %s\n", err)
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(r.TimeStamp, r.Address, r.ReceiveSite, r.ReceiveHost, r.RTT,
		r.Type, r.Code, r.RequestID, r.Sequence, r.DataMatch); err != nil {
		log.Printf("ERROR: executing Result transaction. %s\n", err)
		return err
	}

	return nil
}
func (r *Result) Commit(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("ERROR: beginning Result transaction. %s\n", err)
		return err
	}

	if err := r.Batch(tx); err != nil {
		if rberr := tx.Rollback(); rberr != nil {
			log.Printf("ERROR: rolling back Result transaction. %s\n", rberr)
		}
		return err
	}
	tx.Commit()

	return nil
}

func (r *Result) String() string {
	return fmt.Sprintf(
		"Id: %d, Timestamp: %s, Address: %s\n"+
			"Type: %d, Code: %d\n"+
			"Id: %d, Seq: %d\n"+
			"Receive Site: %d, Receive Host: %d, RTT: %d\n"+
			"DataMatch: %t\n",
		r.Id,
		time.Unix(0, r.TimeStamp),
		r.Address,
		r.Type,
		r.Code,
		r.RequestID,
		r.Sequence,
		r.ReceiveSite,
		r.ReceiveHost,
		r.RTT,
		r.DataMatch)
}

func BatchResultWriter(results []*Result, sqldb *sql.DB) error {
	// Given a collection of Result struct, commit them as a single
	// batch in a single begin/end tran instead of as individual
	// transactions
	tx, err := sqldb.Begin()
	if err != nil {
		log.Printf("ERROR: beginning batch Result transaction. %s\n", err)
		return err
	}

	for _, result := range results {
		if err := result.Batch(tx); err != nil {
			if rberr := tx.Rollback(); rberr != nil {
				log.Printf("ERROR: rolling back Result transaction. %s\n", rberr)
			}
			return err
		}
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

func GetResults(db *sql.DB) []*Result {
	var results []*Result
	sqlstmnt := `SELECT id, rtime, address, rsite, rhost, rtt, rtype, rcode, rid, rseq, datamatch FROM results`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: querying Results. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		r := Result{}
		err = rows.Scan(&r.Id, &r.TimeStamp, &r.Address, &r.ReceiveSite, &r.ReceiveHost, &r.RTT,
			&r.Type, &r.Code, &r.RequestID, &r.Sequence, &r.DataMatch)
		if err != nil {
			log.Printf("ERROR: querying Results. %s\n", err)
			return nil
		}
		results = append(results, &r)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR: querying Results. %s\n", err)
		return nil
	}

	return results
}
