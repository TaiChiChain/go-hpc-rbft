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

func TestMultiBackends(t *testing.T) {
	tests := []struct {
		expected *multiBackends
		backends []Backend
	}{
		{
			&multiBackends{
				[]Backend{
					NewNopBackend(),
					NewIOBackend(&JSONFormatter{"", true, FieldMap{}, false}, os.Stderr),
				}},
			[]Backend{
				NewNopBackend(),
				NewIOBackend(&JSONFormatter{"", true, FieldMap{}, false}, os.Stderr),
			},
		},
	}

	for _, test := range tests {
		backends := MultiBackends(test.backends[0], test.backends[1])
		assert.Equal(t, test.expected, backends)
	}

	assert.Equal(t, NewNopBackend(), MultiBackends())
}

func TestWrite(t *testing.T) {
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
	file, ferr := os.Create("./multi_test_write.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./multi_test_write.txt")
	}()
	assert.Nil(t, ferr)

	backend := multiBackends{[]Backend{NewIOBackend(&JSONFormatter{"", true,
		FieldMap{}, false}, buf),
		NewIOBackend(&JSONFormatter{"", true,
			FieldMap{}, false}, file)}}

	err := backend.Write(test.record)
	assert.Nil(t, err)

	b := &bytes.Buffer{}
	encoder := json.NewEncoder(b)
	ecerr := encoder.Encode(test.expected)
	assert.Nil(t, ecerr)
	assert.Equal(t, b.Bytes(), test.record.Buffer.Bytes())

	file1, err := os.OpenFile("./multi_test_write.txt", os.O_RDONLY, 0777)
	// nolint:errcheck
	defer func() {
		_ = file1.Close()
	}()
	assert.Nil(t, err)

	fileInfo, err := file.Stat()
	assert.Nil(t, err)

	fileSize := fileInfo.Size()
	bf := make([]byte, fileSize)
	_, err = file1.Read(bf)
	assert.Nil(t, err)
	assert.Equal(t, b.Bytes(), bf)
}

func TestMultiHooks(t *testing.T) {
	assert.Equal(t, NewNopHook(), MultiHooks())
	assert.Equal(t, &multiHooks{mHooks: []Hook{NewNopHook(), NewLevelHooks()}},
		MultiHooks(NewNopHook(), NewLevelHooks()))
}

func TestMultiHooks_Fire(t *testing.T) {
	mh := MultiHooks(&TestHook{false, new(bytes.Buffer)})

	record := &Record{
		Module:       "test",
		Level:        ERROR,
		Fields:       make(map[string]interface{}),
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       CallerInfo{},
		Message:      "a new message",
		Buffer:       bytes.NewBuffer(nil),
	}

	_ = mh.Fire(record)

	assert.Equal(t, true, mh.(*multiHooks).mHooks[0].(*TestHook).Fired)
	assert.Equal(t, record.Buffer, mh.(*multiHooks).mHooks[0].(*TestHook).buf)
}
