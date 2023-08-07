package mempool

import (
	"testing"

	"github.com/axiomesh/axiom-bft/common/consensus"

	"github.com/stretchr/testify/assert"
)

func TestRequestHashBatch_IsConfBatch(t *testing.T) {
	testRequestHashBatchIsConfBatch[consensus.FltTransaction](t)
}

func testRequestHashBatchIsConfBatch[T any, Constraint consensus.TXConstraint[T]](t *testing.T) {
	tx1 := &consensus.FltTransaction{TxType: consensus.FltTransaction_CTX}
	txBytes1, err := tx1.Marshal()
	assert.Nil(t, err)
	batch1 := &RequestHashBatch[T, Constraint]{
		BatchHash:  "",
		TxHashList: nil,
		TxList:     [][]byte{txBytes1},
		LocalList:  nil,
		Timestamp:  0,
	}
	flag1 := batch1.IsConfBatch()

	tx2 := &consensus.FltTransaction{TxType: consensus.FltTransaction_NTX}
	txBytes2, err := tx2.Marshal()
	assert.Nil(t, err)
	batch2 := &RequestHashBatch[T, Constraint]{
		BatchHash:  "",
		TxHashList: nil,
		TxList:     [][]byte{txBytes2},
		LocalList:  nil,
		Timestamp:  0,
	}
	flag2 := batch2.IsConfBatch()

	assert.Equal(t, flag1, true)
	assert.Equal(t, flag2, false)
}
