package consensus

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/meshplus/bitxhub-kit/types"
)

func newTx() *Transaction {
	return &Transaction{
		Value: []byte(string(rune(rand.Int()))),
		Nonce: int64(rand.Int()),
	}
}
func TestTransaction_GetHash(t *testing.T) {
	tx := newTx()
	h := tx.GetHash()
	fmt.Println(h)
}

func TestName(t *testing.T) {
	h := types.NewHash([]byte("feawgfhivbahgfaliughaiorghaeioghrevaeavew"))
	fmt.Println(h)
}
