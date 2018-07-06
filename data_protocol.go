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
	"bytes"
	"encoding/binary"
	"log"
)

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

type Magic uint8

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
