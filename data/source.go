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
type Source struct {
	ID             int
	SourceLocation uint32
	SourceHost     uint32
	SourceID       uint16
	Address        string
}

func (r *Source) String() string {
	return fmt.Sprintf("Id: %d, Location: %d, Host: %d, Source Id: %d, Address: %s\n",
		r.ID, r.SourceLocation, r.SourceHost, r.SourceID, r.Address)
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

func GetSources(db *sql.DB) []*Source {
	var sources []*Source
	sqlstmnt := `SELECT id, location, host, sourceid, address FROM sources`

	rows, err := db.Query(sqlstmnt)
	if err != nil {
		log.Printf("ERROR: querying sources. %s\n", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		s := Source{}
		err = rows.Scan(&s.ID, &s.SourceLocation, &s.SourceHost, &s.SourceID, &s.Address)
		if err != nil {
			log.Printf("ERROR: querying sources. %s\n", err)
			return nil
		}
		sources = append(sources, &s)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("ERROR: querying sources. %s\n", err)
		return nil
	}

	return sources
}
