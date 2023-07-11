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
	"fmt"
)

type fieldKey string

// FieldMap records the default field key maps.
type FieldMap map[fieldKey]string

func (f FieldMap) resolve(key fieldKey) string {
	if k, ok := f[key]; ok {
		return k
	}

	return string(key)
}

// JSONFormatter implements the Formatter interface using json-format output.
type JSONFormatter struct {
	// TimestampFormat specify the output timestamp format.
	TimestampFormat string

	// DisableTimestamp indicates whether disable printing timestamp or not.
	DisableTimestamp bool

	// FieldMap records the filtered key if users specified some conflict keys in Records.
	FieldMap FieldMap

	// PrettyPrint indicates whether pretty print json or not.
	PrettyPrint bool
}

// Format implements the Formatter interface.
func (jf *JSONFormatter) Format(record *Record) ([]byte, error) {
	data := make(Fields, len(record.Fields)+len(record.Tags)+5)
	for k, v := range record.Fields {
		data[k] = v
	}
	for k, v := range record.Tags {
		data[k] = v
	}
	prefixFieldClashes(data, jf.FieldMap, record.EnableCaller)
	timestampFormat := jf.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

	if !jf.DisableTimestamp {
		data[jf.FieldMap.resolve(FieldKeyTime)] = record.Time.Format(timestampFormat)
	}

	data[jf.FieldMap.resolve(FieldKeyModule)] = record.Module
	// TRACE level need not be written.
	if record.Level != TRACE {
		data[jf.FieldMap.resolve(FieldKeyLevel)] = record.Level.String()
	}
	if record.Message != "" {
		data[jf.FieldMap.resolve(FieldKeyMsg)] = record.Message
	}
	if record.EnableCaller {
		data[jf.FieldMap.resolve(FieldKeyFunc)] = record.Caller.TrimmedPath()
	}

	var b *bytes.Buffer
	if record.Buffer != nil {
		b = record.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	encoder := json.NewEncoder(b)
	if jf.PrettyPrint {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(data); err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON, %v", err)
	}

	return b.Bytes(), nil
}
