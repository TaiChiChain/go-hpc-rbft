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
	"fmt"
	"strings"
)

// Level defines all available log levels for log messages.
type Level int

// Log levels.
const (
	TRACE Level = iota
	CRITICAL
	ERROR
	WARNING
	NOTICE
	INFO
	DEBUG
)

var levelNames = []string{
	"TRACE",
	"CRITICAL",
	"ERROR",
	"WARNING",
	"NOTICE",
	"INFO",
	"DEBUG",
}

// String returns the string representation of a logging level.
func (l Level) String() string {
	return levelNames[l]
}

// ParseLevel takes a string level and returns the log level constant.
func ParseLevel(l string) (Level, error) {
	switch strings.ToLower(l) {
	case "trace":
		return TRACE, nil
	case "critical":
		return CRITICAL, nil
	case "error":
		return ERROR, nil
	case "warning":
		return WARNING, nil
	case "notice":
		return NOTICE, nil
	case "info":
		return INFO, nil
	case "debug":
		return DEBUG, nil
	}

	return ERROR, fmt.Errorf("invalid Level: %q", l)
}

// Enabled returns if the given level is available under current level.
func (l Level) Enabled(lvl Level) bool {
	return lvl <= l
}

// ContainsLevel returns if the given level list ls contains l or not.
func ContainsLevel(ls []Level, l Level) bool {
	for _, level := range ls {
		if level == l {
			return true
		}
	}
	return false
}
