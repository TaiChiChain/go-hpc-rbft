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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCallInfo(t *testing.T) {
	info1 := GetCallInfo(999)
	assert.Equal(t, CallerInfo{"???", 0}, info1)

	info2 := GetCallInfo(0)

	str, _ := os.Getwd()
	assert.Equal(t, CallerInfo{str + "/record.go", 67}, info2)
}

func TestCallerInfo_String(t *testing.T) {
	test := struct {
		expected string
		ec       CallerInfo
	}{
		"file:10",
		CallerInfo{"file", 10},
	}

	fp := test.ec.String()
	assert.Equal(t, test.expected, fp)
}

func TestCallerInfo_TrimmedPath(t *testing.T) {
	tests := []struct {
		expected string
		ec       CallerInfo
	}{
		{
			"C:10",
			CallerInfo{"C", 10},
		},
		{
			"B/C:10",
			CallerInfo{"B/C", 10},
		},
		{
			"B/C:10",
			CallerInfo{"A/B/C", 10},
		},
	}

	for _, test := range tests {
		tp := test.ec.TrimmedPath()
		assert.Equal(t, test.expected, tp)
	}
}
