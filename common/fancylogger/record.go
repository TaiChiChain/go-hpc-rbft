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
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Record is the actual log entry which will be printed by Backend using
// the given Formatter.
type Record struct {
	// Module is the module name.
	Module string

	// Level is the current record level rather than the default level of fancyLogger.
	Level Level

	// Time is the timestamp when user prints the record.
	Time time.Time

	// EnableCaller indicates whether to print caller info or not.
	EnableCaller bool

	// Caller is the detailed caller info.
	Caller CallerInfo

	// Fields is the default fields of fancyLogger which will be printed on every log entry.
	Fields Fields

	// Tags is the fields of fancyLogger which will be printed on Trace loggers.
	Tags Fields

	// Message is the actual log message.
	Message string

	// Buffer is used to record formatted byte stream.
	Buffer *bytes.Buffer
}

// CallerInfo defines the caller info struct.
type CallerInfo struct {
	// the printing file of current log entry.
	File string

	// the printing line of current log entry.
	Line int
}

// GetCallInfo return the caller info according call depth.
func GetCallInfo(depth int) CallerInfo {
	_, file, line, ok := runtime.Caller(depth)
	if !ok {
		return CallerInfo{"???", 0}
	}
	return CallerInfo{file, line}
}

// String implements the Stringer interface.
func (ec CallerInfo) String() string {
	return ec.FullPath()
}

// FullPath returns the full path of caller info.
func (ec CallerInfo) FullPath() string {
	//TODO: use buffer operation?
	fullPath := ec.File + ":" + strconv.Itoa(ec.Line)

	return fullPath
}

// TrimmedPath returns the trimmed path of caller info.
func (ec CallerInfo) TrimmedPath() string {
	idx := strings.LastIndexByte(ec.File, '/')
	if idx == -1 {
		return ec.FullPath()
	}
	idx = strings.LastIndexByte(ec.File[:idx], '/')
	if idx == -1 {
		return ec.FullPath()
	}
	trimmedPath := ec.File[idx+1:] + ":" + strconv.Itoa(ec.Line)

	return trimmedPath
}
