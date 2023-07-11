// Copyright 2016-2017 Hyperchain Corp.
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

package txpool

import "errors"

// Errors holds package-level variables that represent different errors related
// to batch and the transaction pool.
var (
	// ErrNoBatch means there is no batch with id
	ErrNoBatch = errors.New("can't find batch with id")

	// ErrMismatch means received mismatch txs for one hash
	ErrMismatch = errors.New("mismatch tx hash after receive return fetch txs")

	// ErrDuplicateTx means the received tx is duplicate
	ErrDuplicateTx = errors.New("duplicate transaction")

	// ErrNilBatch means txBatch is nil
	ErrNilBatch = errors.New("txBatch is nil")

	// ErrNil means that there was a nil pointer dereference
	ErrNil = errors.New("nil pointer dereference")

	// ErrNotFoundElement means no element found for specified key
	ErrNotFoundElement = errors.New("not found element")

	// ErrMismatchElement means found a duplicate element with different key
	ErrMismatchElement = errors.New("found mismatch element")

	// ErrInvalidBatch means found an invalid batch
	ErrInvalidBatch = errors.New("invalid batch")

	// ErrInvalidTx means there is an invalid tx
	ErrInvalidTx = errors.New("the tx in nonBatch is not legal")
)
