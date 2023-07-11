package consensus

import "github.com/meshplus/bitxhub-model/pb"

// TXConstraint is used to ensure that the pointer of T must be RbftTransaction
type TXConstraint[T any] interface {
	*T
	pb.RbftTransaction
}

type Transactions = []pb.RbftTransaction

func DecodeTx[T any, Constraint TXConstraint[T]](raw []byte) (*T, error) {
	var t T
	if err := Constraint(&t).RbftUnmarshal(raw); err != nil {
		return nil, err
	}
	return &t, nil
}

// todo: not support temporary

// IsConfigTx returns if this tx is corresponding with a config tx.
func IsConfigTx(_ []byte) bool {
	return false
}
