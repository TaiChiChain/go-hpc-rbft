package consensus

import (
	"fmt"
	"math/rand"
	"testing"
)

func newTx() *Transaction {
	return &Transaction{
		Value: []byte(string(rune(rand.Int()))),
		Nonce: int64(rand.Int()),
	}
}
func TestTransaction_GetHash(t *testing.T) {
	tx := newTx()
	h := tx.RbftGetTxHash()
	fmt.Println(h)
}
