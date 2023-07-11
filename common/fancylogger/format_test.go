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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefixFieldClashes(t *testing.T) {
	test := struct {
		expected     Fields
		data         Fields
		fieldMap     FieldMap
		reportCaller bool
	}{
		Fields{
			"fields.module":   "test",
			"fields.level":    ERROR,
			"fields.message":  "Test Message",
			"fields.time":     "2019/04/01",
			"fields.function": "TestPrefixFieldClashes",
			"function":        "TestPrefixFieldClashes",
		},
		Fields{
			"module":   "test",
			"level":    ERROR,
			"message":  "Test Message",
			"time":     "2019/04/01",
			"function": "TestPrefixFieldClashes",
		},
		FieldMap{
			"module": "module",
			"level":  "level",
			"msg":    "message",
			"time":   "time",
			"func":   "function",
		},
		true,
	}

	prefixFieldClashes(test.data, test.fieldMap, test.reportCaller)
	assert.EqualValues(t, test.expected, test.data)
}
