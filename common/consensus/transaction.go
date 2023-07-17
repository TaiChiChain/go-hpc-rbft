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
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/pingcap/failpoint"
	"golang.org/x/crypto/sha3"
)

var _ pb.Transaction = (*Transaction)(nil)

const (
	// TxVersion35 tx version 3.5
	TxVersion35 = "3.5"
	TimeLength  = 8
	HashLength  = 32
)

func (m *Transaction) RbftGetData() []byte {
	return m.Value
}

// IsConfigTx returns if this tx is corresponding with a config tx.
func (m *Transaction) IsConfigTx() bool {
	return m.GetTxType() == Transaction_CTX || m.GetTxType() == Transaction_ANCHORTX ||
		m.GetTxType() == Transaction_ANCHORTXAUTO || m.GetTxType() == Transaction_TIMEOUTTX
}

func (m *Transaction) GetNonce() uint64 {
	return uint64(m.Nonce)
}

// GetValue just for test
func (m *Transaction) GetValue() *big.Int {
	return nil
}

func (m *Transaction) GetSignature() []byte {
	return m.Signature
}

func (m *Transaction) GetTxType() Transaction_TxType {
	return m.TxType
}

func (m *Transaction) GetVersion() []byte {
	return m.Version
}

func (m *Transaction) GetFrom() *types.Address {
	return types.NewAddressByStr(string(m.From))
}

func (m *Transaction) GetTo() *types.Address {
	return types.NewAddressByStr(string(m.To))
}

func (m *Transaction) GetPayload() []byte {
	return m.Value
}

func (m *Transaction) GetTimeStamp() int64 {
	return m.Timestamp
}

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
func hash(tx *Transaction) []byte {
	var h32 [32]byte
	if CompareVersion(string(tx.Version), TxVersion35) > 0 {
		binary.BigEndian.PutUint64(h32[0:TimeLength], uint64(tx.GetTimeStamp()))
		copy(h32[TimeLength:], tx.Signature[len(tx.Signature)-24:])
		return h32[:]
	}
	res, jerr := json.Marshal([]interface{}{
		tx.From,
		tx.To,
		tx.Value,
		tx.Timestamp,
		tx.Nonce,
		tx.Signature,
		tx.ExpirationTimestamp,
		tx.Participant,
	})

	failpoint.Inject("fp-Hash-1", func() {
		jerr = errors.New("Hash : excepted error")
	})

	if jerr != nil {
		// copy the history logic
		panic(jerr)
	}
	hasher := sha3.NewLegacyKeccak256()
	_, herr := hasher.Write(res)
	failpoint.Inject("fp-Hash-2", func() {
		herr = errors.New("Hash : excepted error")
	})
	if herr != nil {
		return nil
	}
	h := hasher.Sum(nil)

	ha := SetBytes(h)
	binary.BigEndian.PutUint64(ha[0:TimeLength], uint64(tx.GetTimeStamp()))
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

// todo: not sure
func (m *Transaction) GetHash() *types.Hash {
	if m.TransactionHash == nil {
		return types.NewHash(hash(m))
	}
	return types.NewHash(m.TransactionHash)
}

func (m *Transaction) GetIBTP() *pb.IBTP {
	return nil
}

func (m *Transaction) GetExtra() []byte {
	return nil
}

func (m *Transaction) GetGas() uint64 {
	return 0
}

func (m *Transaction) GetGasPrice() *big.Int {
	return nil
}

func (m *Transaction) GetChainID() *big.Int {
	return big.NewInt(0)
}

// GetRawSignature not supprot
func (m *Transaction) GetRawSignature() (*big.Int, *big.Int, *big.Int) {
	return nil, nil, nil
}

// GetSignHash just for test
func (m *Transaction) GetSignHash() *types.Hash {
	return m.GetHash()
}

func (m *Transaction) GetType() byte {
	return byte(m.GetTxType())
}

func (m *Transaction) IsIBTP() bool {
	return false
}

func (m *Transaction) MarshalWithFlag() ([]byte, error) {
	return m.Marshal()
}

func (m *Transaction) SizeWithFlag() int {
	return m.Size()
}

// VerifySignature just for test
func (m *Transaction) VerifySignature() error {
	return nil
}

func (m *Transaction) RbftGetTxHash() string {
	return m.GetHash().String()
}

func (m *Transaction) RbftGetFrom() string {
	return m.GetFrom().String()
}

func (m *Transaction) RbftGetTimeStamp() int64 {
	return m.GetTimeStamp()
}

func (m *Transaction) RbftGetNonce() uint64 {
	return m.GetNonce()
}

func (m *Transaction) RbftUnmarshal(raw []byte) error {
	return m.Unmarshal(raw)
}

func (m *Transaction) RbftMarshal() ([]byte, error) {
	return m.Marshal()
}

func (m *Transaction) RbftIsConfigTx() bool {
	return m.IsConfigTx()
}

func (m *Transaction) RbftGetSize() int {
	return m.Size()
}
