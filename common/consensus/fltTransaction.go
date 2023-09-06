// Copyright 2016-2022 Hyperchain Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consensus

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"

	"golang.org/x/crypto/sha3"
)

var _ RbftTransaction = (*FltTransaction)(nil)

type RbftTransaction interface {
	RbftGetTxHash() string
	RbftGetFrom() string
	RbftGetTimeStamp() int64
	RbftGetData() []byte
	RbftGetNonce() uint64
	RbftUnmarshal(raw []byte) error
	RbftMarshal() ([]byte, error)
	RbftIsConfigTx() bool
	RbftGetSize() int
}

const (
	// TxVersion35 tx version 3.5
	TxVersion35 = "3.5"
	TimeLength  = 8
	HashLength  = 32
)

// CompareVersion compares the value of a and b , return 1 if a > b ,0 if a = b, -1 if a < b
func CompareVersion(a, b string) int {
	indexa, indexb := 0, 0
	parta, partb := 0, 0
	lengtha, lengthb := len(a), len(b)
	f := func(str string, index, length int) (int, int) {
		part := 0
		for ; index < length; index++ {
			if str[index] != '.' {
				part = part << 4
				part += int(str[index] - '0')
			} else {
				index++
				break
			}
		}
		return part, index
	}

	for indexa < lengtha || indexb < lengthb {
		parta, indexa = f(a, indexa, lengtha)
		partb, indexb = f(b, indexb, lengthb)
		if parta > partb {
			return 1
		} else if parta < partb {
			return -1
		}
	}
	return 0
}

// Hash returns the transaction hash calculated using Keccak256.
func hash(tx *FltTransaction) []byte {
	var h32 [32]byte
	if CompareVersion(string(tx.Version), TxVersion35) > 0 {
		binary.BigEndian.PutUint64(h32[0:TimeLength], uint64(tx.RbftGetTimeStamp()))
		copy(h32[TimeLength:], tx.Signature[len(tx.Signature)-24:])
		return h32[:]
	}
	res, jerr := json.Marshal([]any{
		tx.From,
		tx.To,
		tx.Value,
		tx.Timestamp,
		tx.Nonce,
		tx.Signature,
		tx.ExpirationTimestamp,
		tx.Participant,
	})

	if jerr != nil {
		// copy the history logic
		panic(jerr)
	}
	hasher := sha3.NewLegacyKeccak256()
	_, herr := hasher.Write(res)
	if herr != nil {
		return nil
	}
	h := hasher.Sum(nil)

	ha := SetBytes(h)
	binary.BigEndian.PutUint64(ha[0:TimeLength], uint64(tx.RbftGetTimeStamp()))
	return ha
}

// SetBytes sets the hash to the value of b. If b is larger than len(h) it will panic
func SetBytes(b []byte) []byte {
	output := make([]byte, HashLength)
	if len(b) < HashLength {
		copy(output[HashLength-len(b):], b)
	} else {
		copy(output, b)
	}
	return output
}

func (m *FltTransaction) RbftGetTxHash() string {
	h := hash(m)
	return "0x" + base64.StdEncoding.EncodeToString(h)
}

func (m *FltTransaction) RbftGetFrom() string {
	return string(m.From)
}

func (m *FltTransaction) RbftGetTimeStamp() int64 {
	return m.Timestamp
}

func (m *FltTransaction) RbftGetData() []byte {
	return m.Value
}

func (m *FltTransaction) RbftGetNonce() uint64 {
	return uint64(m.Nonce)
}

func (m *FltTransaction) RbftUnmarshal(raw []byte) error {
	return m.Unmarshal(raw)
}

func (m *FltTransaction) RbftMarshal() ([]byte, error) {
	return m.Marshal()
}

func (m *FltTransaction) RbftIsConfigTx() bool {
	return m.TxType == FltTransaction_CTX || m.TxType == FltTransaction_ANCHORTX ||
		m.TxType == FltTransaction_ANCHORTXAUTO || m.TxType == FltTransaction_TIMEOUTTX
}

func (m *FltTransaction) RbftGetSize() int {
	return m.Size()
}
