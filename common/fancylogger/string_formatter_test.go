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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringFormatter_Format(t *testing.T) {
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	var callerInfo CallerInfo
	record := &Record{
		Module:       "test",
		Level:        ERROR,
		Fields:       fields,
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       callerInfo,
		Message:      "a test message",
		Buffer:       bytes.NewBuffer(nil),
	}

	levelColor := ColorSeq(ColorRed) //ERROR: ColorSeq(ColorRed),
	levelText := strings.ToUpper(record.Level.String())
	record.Message = strings.TrimSuffix(record.Message, "\n")

	test := struct {
		expected string
		record   *Record
	}{
		"level=ERROR module=test msg=\"a test message\" key1=value1 key2=value2 key3=value3\n",
		record,
	}
	var result []byte

	// DisableSorting must be true to assure that expected and Format function output in same order
	formatter := &StringFormatter{false, true,
		"", false, true,
		false, true, FieldMap{}}

	result, _ = formatter.Format(test.record)
	expected := []byte(test.expected)
	assert.Equal(t, expected, result)

	// Reset Buffer
	test.record.Buffer.Reset()

	// Tests for func "printColored"
	// EnableColors: true
	formatter1 := &StringFormatter{true, true,
		"", false, false,
		false, true, FieldMap{}}

	// for LevelTruncation started
	levelText = levelText[0:4]

	// for Caller enabled
	record.EnableCaller = true
	caller := record.Caller.TrimmedPath()

	result, _ = formatter1.Format(test.record)
	b := bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(b, "%s%s [%s] %s %s", levelColor, levelText, record.Module, caller, record.Message)
	_, _ = fmt.Fprintf(b, " key1=value1 key2=value2 key3=value3")
	b.WriteByte('\n')
	_, _ = fmt.Fprintf(b, "\033[0m")
	assert.Equal(t, b.Bytes(), result)

	// To reset record.EnableCaller & caller
	record.EnableCaller = false

	// Reset Buffer
	b.Reset()
	test.record.Buffer.Reset()

	// Test for EnableColors&Timestamp
	formatter1.DisableTimestamp = false
	result, _ = formatter1.Format(test.record)

	b = bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(b, "%s%s [%s] [%s] %s", levelColor, levelText, record.Time.Format(defaultTimestampFormat), record.Module, record.Message)
	_, _ = fmt.Fprintf(b, " key1=value1 key2=value2 key3=value3")
	b.WriteByte('\n')
	_, _ = fmt.Fprintf(b, "\033[0m")
	assert.Equal(t, b.Bytes(), result)

	// Reset Buffer
	b.Reset()
	test.record.Buffer.Reset()

	// Test for DisableColors&Timestamp
	formatter1.EnableColors = false
	result, _ = formatter1.Format(test.record)

	b = bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(b, "level=ERROR ")
	_, _ = fmt.Fprintf(b, "time=\"%s\" ", record.Time.Format(defaultTimestampFormat))
	_, _ = fmt.Fprintf(b, "module=test msg=\"a test message\" key1=value1 key2=value2 key3=value3\n")
	assert.Equal(t, b.Bytes(), result)
	b.Reset()
	test.record.Buffer.Reset()

	formatter1.DisableTimestamp = true
	result, _ = formatter1.Format(test.record)

	b = bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(b, "level=ERROR module=test msg=\"a test message\" key1=value1 key2=value2 key3=value3\n")
	assert.Equal(t, b.Bytes(), result)

	b.Reset()
	test.record.Buffer.Reset()

	field := map[string]interface{}{
		"key": "value",
	}

	// Test for DisableSorting
	formatter1.DisableSorting = true
	test.record.Fields = field
	test.record.Buffer = nil
	result, _ = formatter1.Format(test.record)

	b = bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(b, "level=ERROR module=test msg=\"a test message\" key=value\n")
	assert.Equal(t, b.Bytes(), result)
	t.Log(1)
	t.Log(string(b.Bytes()))
	t.Log(2)
	t.Log(string(result))

	b.Reset()
	test.record.Fields = fields
	test.record.Buffer = &bytes.Buffer{}
	test.record.Buffer.Reset()
}

// Test for needsQueting line:202
func TestStringFormatter_Format3(t *testing.T) {
	var sf StringFormatter
	sf.QuoteEmptyFields = true
	assert.Equal(t, true, sf.needsQuoting(""))
}
