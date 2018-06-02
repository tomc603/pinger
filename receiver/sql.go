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
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DbSource struct {
	SourceID int
	Address  string
}

func (ds *DbSource) String() string {
	return fmt.Sprintf("Source ID: %d - Address: %s\n", ds.SourceID, ds.Address)
}

func (ds *DbSource) Commit(db *sql.DB) error {
	sqlstmnt := `INSERT INTO sources(sourceid, address) values(?, ?)`
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
	sqlstmnt := `CREATE TABLE destinations (
		id integer not null primary key,
		sourceid int not null, address text not null);`

	_, err := db.Exec(sqlstmnt)
	if err != nil {
		return err
	}

	return nil
}

func GetSources(db *sql.DB) []DbSource {
	var sources []DbSource
	sqlstmnt := `SELECT sourceid, address FROM sources`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR querying sources. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		s := DbSource{}
		err = rows.Scan(&s.SourceID, &s.Address)
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
	sqlstmnt := `CREATE TABLE destinations (
		id integer not null primary key,
		active bool, address text not null, protocol int not null,
		interval int not null, timeout int not null, ttl int,
		data blob);`

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
		err = rows.Scan(&d.Active, &d.Address, &d.Protocol, &d.Interval,
			&d.Timeout, &d.TTL, &d.Data)
		if err != nil {
			log.Printf("ERROR querying destinations. %s\n", err)
			return nil
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
	sqlstmnt := `CREATE TABLE results (
		id integer not null primary key,
		address text not null, rtime timestamp not null,
		rtype int not null, rcode int not null,
		rid int not null, rseq int not null,
		datamatch bool);`

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
