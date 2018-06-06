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

type Metrics struct {
	sync.RWMutex
	v4Sent          uint
	v4ReceiveFailed uint
	v4ParseFailed   uint
	v4Bytes         uint
	v6Sent          uint
	v6ReceiveFailed uint
	v6ParseFailed   uint
	v6Bytes         uint
	startTime     time.Time
}

func (m *Metrics) Addv4Received(i uint) {
	m.Lock()
	m.v4Sent += i
	m.Unlock()
}

func (m *Metrics) Addv4ReceiveFailed(i uint) {
	m.Lock()
	m.v4ReceiveFailed += i
	m.Unlock()
}

func (m *Metrics) Addv4ParseFailed(i uint) {
	m.Lock()
	m.v4ParseFailed += i
	m.Unlock()
}

func (m *Metrics) Addv4Bytes(i uint) {
	m.Lock()
	m.v4Bytes += i
	m.Unlock()
}

func (m *Metrics) Addv6Received(i uint) {
	m.Lock()
	m.v6Sent += i
	m.Unlock()
}

func (m *Metrics) Addv6ReceiveFailed(i uint) {
	m.Lock()
	m.v6ReceiveFailed += i
	m.Unlock()
}

func (m *Metrics) Addv6ParseFailed(i uint) {
	m.Lock()
	m.v6ParseFailed += i
	m.Unlock()
}

func (m *Metrics) Addv6Bytes(i uint) {
	m.Lock()
	m.v6Bytes += i
	m.Unlock()
}

func (m *Metrics) String() string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("Uptime: %v\n" +
		"IPv4 received: %d\n" +
		"IPv4 receive error: %d\n" +
		"IPv4 parse error: %d\n" +
		"IPv4 bytes: %d\n" +
		"IPv6 received: %d\n" +
		"IPv6 receive error: %d\n" +
		"IPv6 parse error: %d\n" +
		"IPv6 bytes: %d\n" +
		"Total received: %d\n" +
		"Total receive error: %d\n" +
		"Total parse error: %d\n" +
		"Total bytes: %d\n",
		time.Since(m.startTime),
		m.v4Sent, m.v4ReceiveFailed, m.v4ParseFailed, m.v4Bytes,
		m.v6Sent, m.v6ReceiveFailed, m.v6ParseFailed, m.v6Bytes,
		m.v4Sent + m.v6Sent,
		m.v4ReceiveFailed + m.v6ReceiveFailed,
		m.v4ParseFailed + m.v6ParseFailed,
		m.v4Bytes + m.v6Bytes)
}
