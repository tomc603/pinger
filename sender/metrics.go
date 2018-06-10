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

func (m *Metrics) Addv4Sent(delta uint) {
	m.Lock()
	m.v4Sent += delta
	m.Unlock()
}

func (m *Metrics) Addv4Failed(delta uint) {
	m.Lock()
	m.v4Failed += delta
	m.Unlock()
}

func (m *Metrics) Addv4Bytes(delta uint) {
	m.Lock()
	m.v4Bytes += delta
	m.Unlock()
}

func (m *Metrics) Addv6Sent(delta uint) {
	m.Lock()
	m.v6Sent += delta
	m.Unlock()
}

func (m *Metrics) Addv6Failed(delta uint) {
	m.Lock()
	m.v6Failed += delta
	m.Unlock()
}

func (m *Metrics) Addv6Bytes(delta uint) {
	m.Lock()
	m.v6Bytes += delta
	m.Unlock()
}

func (m *Metrics) AddEmptyDest(delta uint) {
	m.Lock()
	m.emptyDest += delta
	m.Unlock()
}

func (m *Metrics) AddDnsTimeout(delta uint) {
	m.Lock()
	m.dnsTimeout += delta
	m.Unlock()
}

func (m *Metrics) AddDnsTempFail(delta uint) {
	m.Lock()
	m.dnsTempFail += delta
	m.Unlock()
}

func (m *Metrics) AddDnsError(delta uint) {
	m.Lock()
	m.dnsError += delta
	m.Unlock()
}

func (m *Metrics) AddAddressError(delta uint) {
	m.Lock()
	m.addrError += delta
	m.Unlock()
}

func (m *Metrics) AddUnknownError(delta uint) {
	m.Lock()
	m.unknownError += delta
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
