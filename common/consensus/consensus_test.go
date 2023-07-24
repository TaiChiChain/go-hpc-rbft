package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeTx(t *testing.T) {
	tx := newTx()
	txBytes, err := tx.RbftMarshal()
	assert.Nil(t, err)
	actTx, err := DecodeTx[FltTransaction](txBytes)
	assert.Nil(t, err)
	assert.Equal(t, tx.Value, actTx.Value)
}

func TestDecodeTxs(t *testing.T) {
	tx1 := newTx()
	tx2 := newTx()
	tx1Bytes, err := tx1.RbftMarshal()
	assert.Nil(t, err)
	tx2Bytes, err := tx2.RbftMarshal()
	assert.Nil(t, err)
	txsBytes := [][]byte{tx1Bytes, tx2Bytes}
	txs, err := DecodeTxs[FltTransaction](txsBytes)
	assert.Nil(t, err)
	assert.Equal(t, tx1.Value, txs[0].Value)
}
