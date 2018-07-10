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
	"log"
	"sync"
)

func resultWriter(resultchan chan Result, sqldb *sql.DB, wg *sync.WaitGroup) {
	var resultBuf []*Result

	wg.Add(1)
	defer wg.Done()

	log.Println("Ping resultWriter started.")
	for result := range resultchan {
		//log.Printf("%s\n", result.String())

		if ResultBatchSize == 0 {
			if ce := result.Commit(sqldb); ce != nil {
				log.Printf("ERROR: Could not commit Result %#v. %s.\n", result, ce)
				db_metrics.AddFailedSingleCommits(1)
			} else {
				db_metrics.AddSingleCommits(1)
			}
		} else if len(resultBuf) >= ResultBatchSize {
			if err := BatchResultWriter(resultBuf, sqldb); err != nil {
				// Commit each Result individually so we save as much data as possible.
				log.Printf("ERROR: Could not commit Result batch. %s.\n", err)
				db_metrics.AddFailedBatchCommits(1)
				for _, r := range resultBuf {
					if ce := r.Commit(sqldb); ce != nil {
						log.Printf("ERROR: Could not commit Result %#v. %s.\n", r, ce)
						db_metrics.AddFailedSingleCommits(1)
					} else {
						db_metrics.AddSingleCommits(1)
					}
				}
			} else {
				db_metrics.AddBatchCommits(1)
			}

			// Empty the result buffer.
			resultBuf = []*Result{}
		} else {
			resultBuf = append(resultBuf, &result)
		}
	}
	log.Println("Ping resultWriter stopped.")
}
