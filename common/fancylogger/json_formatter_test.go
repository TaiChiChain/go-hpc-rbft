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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJSONFormatter_Format(t *testing.T) {
	fields := map[string]interface{}{
		"key2": "value2",
		"key1": "value1",
		"key3": "value3",
	}

	var callerInfo CallerInfo
	record := &Record{
		Module:       "Unit Test",
		Level:        ERROR,
		Fields:       fields,
		Time:         time.Now(),
		EnableCaller: true,
		Caller:       callerInfo,
		Message:      "a test message by JSONFormatter",
		Buffer:       bytes.NewBuffer(nil),
	}

	test := struct {
		expected map[string]interface{}
		record   *Record
	}{
		map[string]interface{}{
			"key2":   "value2",
			"key1":   "value1",
			"key3":   "value3",
			"time":   record.Time.Format(defaultTimestampFormat),
			"module": "Unit Test",
			"level":  "ERROR",
			"msg":    "a test message by JSONFormatter",
			"func":   record.Caller.TrimmedPath(),
		},
		record,
	}

	formatter := &JSONFormatter{"", false, FieldMap{}, true}
	record.Buffer = nil
	result, _ := formatter.Format(test.record)

	b := &bytes.Buffer{}
	// there's some difference between encoder and Marshal method
	encoder := json.NewEncoder(b)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(test.expected)
	assert.Nil(t, err)
	assert.Equal(t, b.Bytes(), result)
}
