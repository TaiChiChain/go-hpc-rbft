package consensus

// TXConstraint is used to ensure that the pointer of T must be RbftTransaction
type TXConstraint[T any] interface {
	*T
	RbftTransaction
}

type Transactions = []RbftTransaction

func DecodeTx[T any, Constraint TXConstraint[T]](raw []byte) (*T, error) {
	var t T
	if err := Constraint(&t).RbftUnmarshal(raw); err != nil {
		return nil, err
	}
	return &t, nil
}

func DecodeTxs[T any, Constraint TXConstraint[T]](rawTxs [][]byte) ([]*T, error) {
	var txs []*T
	for _, rawTx := range rawTxs {
		tx, err := DecodeTx[T, Constraint](rawTx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func EncodeTxs[T any, Constraint TXConstraint[T]](txs []*T) ([][]byte, error) {
	var rawTxs [][]byte
	for _, rawTx := range txs {
		tx, err := Constraint(rawTx).RbftMarshal()
		if err != nil {
			return nil, err
		}
		rawTxs = append(rawTxs, tx)
	}
	return rawTxs, nil
}
