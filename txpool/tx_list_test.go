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

import (
	"container/list"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTxList(t *testing.T) {
	ast := assert.New(t)
	txList := NewTxList()
	ast.Equal(0, txList.Len(), "no tx, expect 0")
	ast.Equal(false, txList.Has("1"), "not exist, expect false")

	txList.PushBack("1", 1)
	ast.Equal(1, txList.Len(), "one tx, expect 1")
	ast.Equal(true, txList.Has("1"), "this tx exists, expect true")
	ast.Equal(1, txList.Get("1").Value.(int), "should be 1")
	ast.Equal(1, txList.Front().Value.(int), "should be 1")

	e := txList.PushFront("0", 0)
	ast.Equal(2, txList.Len(), "two tx, expect 2")
	ast.Equal(0, txList.Front().Value.(int), "should be 0")

	txList.InsertAfter("3", 3, e)
	e4 := txList.InsertBefore("4", 4, e)
	ast.Equal(4, txList.Len(), "four txs, expect 4")
	ast.Equal(4, txList.Front().Value.(int), "should be 4")

	_ = txList.Remove(e4, "4")
	ast.Equal(0, txList.Front().Value.(int), "should be 0")

	_ = txList.Remove(e, "0")
	ast.Equal(3, txList.Front().Value.(int), "should be 3")
}

func TestDuplicateTx(t *testing.T) {

	ast := assert.New(t)
	txList := NewTxList()
	ast.Equal(0, txList.Len(), "List is empty, expect 0.")
	ast.Equal(false, txList.Has("1"), "List is empty, expect false.")

	e := txList.PushFront("1", 1)
	ast.Equal(1, txList.Len(), "One elements, expect 1.")
	ast.Equal(1, txList.Front().Value.(int), "Should be 1.")
	ast.Equal(1, e.Value.(int), "Should be 1.")

	txList.PushFront("1", 1)
	ast.Equal(1, txList.Len(), "One elements, expect 1.")
	ast.Equal(1, txList.Front().Value.(int), "Should be 1.")
	ast.Equal(1, e.Value.(int), "Should be 1.")

	err1 := txList.Remove(txList.Get("1"), "1")
	if err1 != nil {
		t.Errorf("not find 1=first key: %s", err1)
	}

	var n *list.Element
	i := 0
	for ef := txList.Front(); i < txList.pool.Len(); ef = n {
		element, ok := ef.Value.(int)
		if !ok {
			t.Error("cannot convert")
		}

		t.Logf("front value is %d", element)
		err2 := txList.Remove(ef, "1")
		ast.Equal(ErrNotFoundElement, err2, "we should not find key with 1")

		i++
	}
}

func TestTxList_Remove_Error(t *testing.T) {
	ast := assert.New(t)
	txList := NewTxList()
	txList.PushBack("1", 1)
	txList.PushBack("2", 2)
	err0 := txList.Remove(nil, "999")
	ast.Equal(ErrNil, err0)
	err1 := txList.Remove(txList.Front(), "999")
	ast.Equal(ErrNotFoundElement, err1)
	err2 := txList.Remove(txList.Front(), "2")
	ast.Equal(ErrMismatchElement, err2)
}
