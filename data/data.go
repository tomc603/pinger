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
	"encoding/binary"
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
	_               = iota
	ProtoUDP4 uint8 = iota
	ProtoUDP6
)

var DataOrder binary.ByteOrder = binary.LittleEndian
var MagicV1 Magic = 146
