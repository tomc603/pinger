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
	"time"
)

type DestinationMetrics struct {
	v4Sent       uint
	v4Failed     uint
	v4Bytes      uint
	v6Sent       uint
	v6Failed     uint
	v6Bytes      uint
	dnsError     uint
	addrError    uint
}

type Metrics struct {
	sync.RWMutex
	v4Sent       uint
	v4Failed     uint
	v4Bytes      uint
	v6Sent       uint
	v6Failed     uint
	v6Bytes      uint
	emptyDest    uint
	dnsTimeout   uint
	dnsTempFail  uint
	dnsError     uint
	addrError    uint
	unknownError uint
	startTime    time.Time
	destMetrics  map[string]DestinationMetrics
}

func (m *Metrics) Addv4Sent(i uint) {
	m.Lock()
	m.v4Sent += i
	m.Unlock()
}

func (m *Metrics) Addv4Failed(i uint) {
	m.Lock()
	m.v4Failed += i
	m.Unlock()
}

func (m *Metrics) Addv4Bytes(i uint) {
	m.Lock()
	m.v4Bytes += i
	m.Unlock()
}

func (m *Metrics) Addv6Sent(i uint) {
	m.Lock()
	m.v6Sent += i
	m.Unlock()
}

func (m *Metrics) Addv6Failed(i uint) {
	m.Lock()
	m.v6Failed += i
	m.Unlock()
}

func (m *Metrics) Addv6Bytes(i uint) {
	m.Lock()
	m.v6Bytes += i
	m.Unlock()
}

func (m *Metrics) AddEmptyDest(i uint) {
	m.Lock()
	m.emptyDest += i
	m.Unlock()
}

func (m *Metrics) AddDnsTimeout(i uint) {
	m.Lock()
	m.dnsTimeout += i
	m.Unlock()
}

func (m *Metrics) AddDnsTempFail(i uint) {
	m.Lock()
	m.dnsTempFail += i
	m.Unlock()
}

func (m *Metrics) AddDnsError(i uint) {
	m.Lock()
	m.dnsError += i
	m.Unlock()
}

func (m *Metrics) AddAddressError(i uint) {
	m.Lock()
	m.addrError += i
	m.Unlock()
}

func (m *Metrics) AddUnknownError(i uint) {
	m.Lock()
	m.unknownError += i
	m.Unlock()
}

func (m *Metrics) String() string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("Uptime: %v\n" +
		"IPv4 sent: %d\n" +
		"IPv4 failed: %d\n" +
		"IPv4 bytes: %d\n" +
		"IPv6 sent: %d\n" +
		"IPv6 failed: %d\n" +
		"IPv6 bytes: %d\n" +
		"Total sent: %d\n" +
		"Total failed: %d\n" +
		"Total bytes: %d\n" +
		"Empty Destination: %d\n" +
		"DNS timeouts: %d\n" +
		"DNS temporary failures: %d\n" +
		"DNS errors: %d\n" +
		"Address errors: %d\n" +
		"Unknown errors: %d\n",
		time.Since(m.startTime),
		m.v4Sent, m.v4Failed, m.v4Bytes,
		m.v6Sent, m.v6Failed, m.v6Bytes,
		m.v4Sent+m.v6Sent, m.v4Failed + m.v6Failed, m.v4Bytes + m.v6Bytes,
		m.emptyDest, m.dnsTimeout, m.dnsTempFail, m.dnsError,
		m.addrError, m.unknownError)
}
