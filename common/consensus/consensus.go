package consensus

// TXConstraint is used to ensure that the pointer of T must be RbftTransaction
type TXConstraint[T any] interface {
	*T
	RbftTransaction
}

type Transactions = []RbftTransaction

func DecodeTx[T any, Constraint TXConstraint[T]](raw []byte) (*T, error) {
	return decodeTx[T, Constraint](raw)
}

func decodeTx[T any, Constraint TXConstraint[T]](raw []byte) (*T, error) {
	var t T
	if err := Constraint(&t).RbftUnmarshal(raw); err != nil {
		return nil, err
	}
	return &t, nil
}

func DecodeTxs[T any, Constraint TXConstraint[T]](rawTxs [][]byte) ([]*T, error) {
	var txs []*T
	for _, rawTx := range rawTxs {
		tx, err := decodeTx[T, Constraint](rawTx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

// IsConfigTx returns if this tx is corresponding with a config tx.
func IsConfigTx[T any, Constraint TXConstraint[T]](txData []byte) bool {
	tx, err := DecodeTx[T, Constraint](txData)
	if err != nil {
		panic(err)
	}
	return Constraint(tx).RbftIsConfigTx()
}

func GetAccount[T any, Constraint TXConstraint[T]](txData []byte) (string, error) {
	tx, err := DecodeTx[T, Constraint](txData)
	if err != nil {
		return "", err
	}
	return Constraint(tx).RbftGetFrom(), nil
}

func GetTxHash[T any, Constraint TXConstraint[T]](txData []byte) (string, error) {
	tx, err := DecodeTx[T, Constraint](txData)
	if err != nil {
		return "", err
	}
	return Constraint(tx).RbftGetTxHash(), nil
}
func GetNonce[T any, Constraint TXConstraint[T]](txData []byte) (uint64, error) {
	tx, err := DecodeTx[T, Constraint](txData)
	if err != nil {
		return 0, err
	}
	return Constraint(tx).RbftGetNonce(), nil
}
