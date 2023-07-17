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

import "time"

// Fields is used to record default fields.
type Fields map[string]interface{}

const (
	defaultTimestampFormat = time.RFC3339

	// FieldKeyModule is the default key of 'module'.
	FieldKeyModule = "module"
	// FieldKeyLevel is the default key of 'level'.
	FieldKeyLevel = "level"
	// FieldKeyMsg is the default key of 'msg'.
	FieldKeyMsg = "msg"
	// FieldKeyTime is the default key of 'time'.
	FieldKeyTime = "time"
	// FieldKeyFunc is the default key of 'func'.
	FieldKeyFunc = "func"
)

// Formatter defines the interface used to format and add color to actual output if needed.
type Formatter interface {
	// Format formats the record and returns the serialized stream bytes.
	Format(record *Record) ([]byte, error)
}

func prefixFieldClashes(data Fields, fieldMap FieldMap, reportCaller bool) {
	moduleKey := fieldMap.resolve(FieldKeyModule)
	if t, ok := data[moduleKey]; ok {
		data["fields."+moduleKey] = t
		delete(data, moduleKey)
	}

	timeKey := fieldMap.resolve(FieldKeyTime)
	if t, ok := data[timeKey]; ok {
		data["fields."+timeKey] = t
		delete(data, timeKey)
	}

	msgKey := fieldMap.resolve(FieldKeyMsg)
	if m, ok := data[msgKey]; ok {
		data["fields."+msgKey] = m
		delete(data, msgKey)
	}

	levelKey := fieldMap.resolve(FieldKeyLevel)
	if l, ok := data[levelKey]; ok {
		data["fields."+levelKey] = l
		delete(data, levelKey)
	}

	if reportCaller {
		funcKey := fieldMap.resolve(FieldKeyFunc)
		if l, ok := data[funcKey]; ok {
			data["fields."+funcKey] = l
		}
	}
}
