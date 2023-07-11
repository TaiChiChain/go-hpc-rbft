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

package fancylogger

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNopBackend_Write(t *testing.T) {
	backend := NewNopBackend()
	record := &Record{}
	err := backend.Write(record)
	assert.Nil(t, err)

}

func TestIoBackend_Write(t *testing.T) {
	logger := NewLogger("test", INFO)
	_ = logger.WithField("key1", "value1")
	var callerInfo CallerInfo
	record := &Record{
		Module:       "test",
		Level:        ERROR,
		Fields:       logger.Data,
		Time:         time.Now(),
		EnableCaller: logger.enableCaller,
		Caller:       callerInfo,
		Message:      "a new message",
		Buffer:       bytes.NewBuffer(nil),
	}

	test := struct {
		expected map[string]interface{}
		record   *Record
	}{
		map[string]interface{}{
			"key1":   "value1",
			"level":  "ERROR",
			"module": "test",
			"msg":    "a new message",
		},
		record,
	}

	buf := bytes.NewBuffer(nil)
	backend := NewIOBackend(&JSONFormatter{"", true,
		FieldMap{}, false}, buf)

	err := backend.Write(test.record)
	assert.Nil(t, err)

	b := &bytes.Buffer{}
	encoder := json.NewEncoder(b)
	err1 := encoder.Encode(test.expected)

	assert.Nil(t, err1)
	assert.Equal(t, b, test.record.Buffer)
	assert.Equal(t, b, buf)
}

func TestCompositebackend(t *testing.T) {
	test1 := struct {
		expected Backend
		backends []Backend
	}{
		&multiBackends{
			[]Backend{
				NewNopBackend(),
				NewIOBackend(&JSONFormatter{"", true, FieldMap{}, false}, os.Stderr),
			}},
		[]Backend{
			NewNopBackend(),
			NewIOBackend(&JSONFormatter{"", true, FieldMap{}, false}, os.Stderr),
		},
	}

	test2 := struct {
		expected Backend
		backends []Backend
	}{
		// Is it better to make *multiBackends be the return type
		NewNopBackend(),
		[]Backend{NewNopBackend()},
	}

	test3 := struct {
		expected Backend
		backends []Backend
	}{
		DefaultBackend,
		[]Backend{},
	}

	backends1 := compositeBackend(test1.backends[0], test1.backends[1])
	assert.Equal(t, test1.expected, backends1)

	backends2 := compositeBackend(test2.backends[0])
	assert.Equal(t, test2.expected, backends2)

	backends3 := compositeBackend()
	assert.Equal(t, test3.expected, backends3)
}
