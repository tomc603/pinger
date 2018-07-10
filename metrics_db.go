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
	"fmt"
	"sync"
)

type DbMetrics struct {
	sync.RWMutex
	dbAddedDestinations      uint
	dbBatchCommits           uint
	dbDestinationReads       uint
	dbFailedBatchCommits     uint
	dbFailedDestinationReads uint
	dbFailedSingleCommits    uint
	dbModifiedDestinations   uint
	dbRemovedDestinations    uint
	dbSingleCommits          uint
}

func (m *DbMetrics) AddBatchCommits(delta uint) {
	m.Lock()
	m.dbBatchCommits += delta
	m.Unlock()
}

func (m *DbMetrics) AddFailedBatchCommits(delta uint) {
	m.Lock()
	m.dbFailedBatchCommits += delta
	m.Unlock()
}

func (m *DbMetrics) AddSingleCommits(delta uint) {
	m.Lock()
	m.dbSingleCommits += delta
	m.Unlock()
}

func (m *DbMetrics) AddFailedSingleCommits(delta uint) {
	m.Lock()
	m.dbFailedSingleCommits += delta
	m.Unlock()
}

func (m *DbMetrics) AddAddedDestinations(delta uint) {
	m.Lock()
	m.dbAddedDestinations += delta
	m.Unlock()
}

func (m *DbMetrics) AddRemovedDestinations(delta uint) {
	m.Lock()
	m.dbRemovedDestinations += delta
	m.Unlock()
}

func (m *DbMetrics) AddDestinationReads(delta uint) {
	m.Lock()
	m.dbDestinationReads += delta
	m.Unlock()
}

func (m *DbMetrics) AddFailedDestinationReads(delta uint) {
	m.Lock()
	m.dbFailedDestinationReads += delta
	m.Unlock()
}

func (m *DbMetrics) AddModifiedDestinationReads(delta uint) {
	m.Lock()
	m.dbModifiedDestinations += delta
	m.Unlock()
}

func (m *DbMetrics) String() string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("Batch Commits: %d\n"+
		"Single Commits: %d\n"+
		"Failed Batch Commits: %d\n"+
		"Failed Single Commits: %d\n"+
		"Destination Reads: %d\n"+
		"Failed Destination Reads: %d\n"+
		"Added Destinations: %d\n"+
		"Removed Destinations: %d\n",
		m.dbBatchCommits,
		m.dbSingleCommits,
		m.dbFailedBatchCommits,
		m.dbFailedSingleCommits,
		m.dbDestinationReads,
		m.dbFailedDestinationReads,
		m.dbAddedDestinations,
		m.dbRemovedDestinations)
}
