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

func TestLevel_String(t *testing.T) {
	// Make sure all levels can be converted from string -> constant -> string
	for _, name := range levelNames {
		level, err := ParseLevel(name)

		assert.Nil(t, err)
		assert.Equal(t, name, level.String())
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		expected Level
		level    string
	}{
		{expected: -1, level: "aaa"},
		{expected: WARNING, level: "waRnIng"},
		{expected: CRITICAL, level: "Critical"},
		{expected: DEBUG, level: "debug"},
	}

	for _, test := range tests {
		level, err := ParseLevel(test.level)

		if err != nil {
			assert.Equal(t, test.expected, Level(-1))
		} else {
			assert.Equal(t, test.expected, level)
		}
	}
}

func TestLevel_Enabled(t *testing.T) {
	tests := []struct {
		expected bool
		l1       Level
		l2       Level
	}{
		{
			true,
			ERROR,
			CRITICAL,
		},
		{
			false,
			INFO,
			DEBUG,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.l1.Enabled(test.l2))
	}
}

func TestContainsLevel(t *testing.T) {
	tests := []struct {
		expected bool
		ls       []Level
		l        Level
	}{
		{
			true,
			[]Level{ERROR, CRITICAL, INFO},
			INFO,
		},
		{
			false,
			[]Level{ERROR, CRITICAL, INFO},
			DEBUG,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, ContainsLevel(test.ls, test.l))
	}
}
