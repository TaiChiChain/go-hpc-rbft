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
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/hyperchain/go-hpc-rbft/common/consensus"
)

const callDepthOffset = 3

var (
	bufferPool *sync.Pool
)

func init() {
	bufferPool = &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
}

// Logger is the actual fancyLogger instance which implements the six-level logger functions:
// DEBUG ==> INFO ==> NOTICE ==> WARNING ==> ERROR ==> CRITICAL
// Every Logger instance has a default logger level and its own Backend.
type Logger struct {
	// module name.
	Module string

	// default printing level, only records with higher level can be printed.
	Level Level

	// default fields which will be printed with every record.
	// users can use WithField and WithFields to add default fields.
	Data Fields

	// print caller function info or not.
	enableCaller bool

	// caller info depth.
	callDepth int

	// Logger output destination.
	Backends      Backend
	TraceBackends Backend

	// actions which will be triggered under specific conditions.
	Hooks Hook

	mu sync.RWMutex
}

// NewLogger creates a Logger instance with module name and default level.
func NewLogger(module string, level Level) *Logger {
	return &Logger{
		Module: module,
		Level:  level,
		// TODO: how many default fields is suitable?
		Data: make(Fields, 6),
	}
}

// NewRawLogger returns a raw logger used in test in which logger messages
// will be printed to Stdout with string formatter.
func NewRawLogger() *Logger {
	rawLogger := NewLogger("test", DEBUG)

	consoleFormatter := &StringFormatter{
		EnableColors:    true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		IsTerminal:      true,
	}
	consoleBackend := NewIOBackend(consoleFormatter, os.Stdout)

	rawLogger.SetBackends(consoleBackend)
	rawLogger.SetTraceBackends(consoleBackend)
	rawLogger.SetEnableCaller(true)

	return rawLogger
}

// GetModule returns the Logger module name.
func (logger *Logger) GetModule() string {
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	return logger.Module
}

// SetModule sets the Logger module name.
func (logger *Logger) SetModule(module string) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Module = module
}

// GetLevel returns the Logger default level.
func (logger *Logger) GetLevel() Level {
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	return logger.Level
}

// SetLevel sets the Logger default level.
func (logger *Logger) SetLevel(level Level) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Level = level
}

// IsLevelEnabled returns if the given level can be printed under the default log level.
func (logger *Logger) IsLevelEnabled(level Level) bool {
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	return logger.Level.Enabled(level)
}

// SetBackends sets the backend.
func (logger *Logger) SetBackends(backends ...Backend) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Backends = compositeBackend(backends...)
}

// SetTraceBackends sets the trace backend.
func (logger *Logger) SetTraceBackends(backends ...Backend) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.TraceBackends = compositeBackend(backends...)
}

// SetHooks sets the hook functions.
func (logger *Logger) SetHooks(hooks ...Hook) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Hooks = compositeHook(hooks...)
}

// IsEnableCaller returns if enable printing caller info or not.
func (logger *Logger) IsEnableCaller() bool {
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	return logger.enableCaller
}

// SetEnableCaller sets enableCaller.
func (logger *Logger) SetEnableCaller(enable bool) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.enableCaller = enable
}

// AddCallDepth increases call depth. usually used when users want to wrapper Logger instance.
func (logger *Logger) AddCallDepth(depth int) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.callDepth += depth
}

// WithField adds one field with key-value pair to default fields.
func (logger *Logger) WithField(key string, value interface{}) error {
	return logger.WithFields(Fields{key: value})
}

// WithFields adds multiply fields with key-value pairs to default fields.
func (logger *Logger) WithFields(fields Fields) error {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	for k, v := range fields {
		isErrField := false
		if t := reflect.TypeOf(v); t != nil {
			switch t.Kind() {
			case reflect.Func:
				isErrField = true
			case reflect.Ptr:
				isErrField = t.Elem().Kind() == reflect.Func
			}
		}
		if isErrField {
			return fmt.Errorf("invalid fields with key=%s, value=%v", k, v)
		}
		logger.Data[k] = v
	}
	return nil
}

func (logger *Logger) write(level Level, msg string) {
	logger.mu.RLock()
	defer logger.mu.RUnlock()

	var callerInfo CallerInfo
	if logger.enableCaller {
		callerInfo = GetCallInfo(logger.callDepth + callDepthOffset)
	}

	record := &Record{
		Module:       logger.Module,
		Level:        level,
		Fields:       logger.Data,
		Time:         time.Now(),
		EnableCaller: logger.enableCaller,
		Caller:       callerInfo,
		Message:      msg,
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer bufferPool.Put(buffer)
	record.Buffer = buffer

	logger.log(record, logger.Backends)
	logger.fireHooks(record)

	buffer = nil
}

func (logger *Logger) writeTrace(name, stage string, content interface{}) {
	logger.mu.RLock()
	defer logger.mu.RUnlock()

	if name == "" || stage == "" {
		return
	}
	tags := make(map[string]interface{}, 3)
	tags[consensus.TagKey] = name
	tags[consensus.TagStageKey] = stage
	if content != nil {
		tags[consensus.TagContentKey] = content
	}

	var callerInfo CallerInfo
	if logger.enableCaller {
		callerInfo = GetCallInfo(logger.callDepth + callDepthOffset)
	}

	record := &Record{
		Module:       logger.Module,
		Level:        TRACE,
		Fields:       logger.Data,
		Tags:         tags,
		Time:         time.Now(),
		EnableCaller: logger.enableCaller,
		Caller:       callerInfo,
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer bufferPool.Put(buffer)
	record.Buffer = buffer

	logger.log(record, logger.TraceBackends)
	logger.fireHooks(record)

	buffer = nil
}

func (logger *Logger) log(record *Record, backend Backend) {
	if backend == nil {
		backend = DefaultBackend
	}

	err := backend.Write(record)
	if err != nil {
		// nolint:errcheck
		fmt.Fprintf(os.Stderr, "Failed to format record, %v\n", err)
	}
}

func (logger *Logger) fireHooks(record *Record) {
	hook := logger.Hooks
	if hook == nil {
		return
	}

	err := hook.Fire(record)
	if err != nil {
		// nolint:errcheck
		fmt.Fprintf(os.Stderr, "Failed to fire hooks, %v\n", err)
	}
}

// Debug prints message with debug level if allowed.
func (logger *Logger) Debug(args ...interface{}) {
	if logger.IsLevelEnabled(DEBUG) {
		logger.write(DEBUG, fmt.Sprint(args...))
	}
}

// Info prints message with info level if allowed.
func (logger *Logger) Info(args ...interface{}) {
	if logger.IsLevelEnabled(INFO) {
		logger.write(INFO, fmt.Sprint(args...))
	}
}

// Notice prints message with notice level if allowed.
func (logger *Logger) Notice(args ...interface{}) {
	if logger.IsLevelEnabled(NOTICE) {
		logger.write(NOTICE, fmt.Sprint(args...))
	}
}

// Warning prints message with warning level if allowed.
func (logger *Logger) Warning(args ...interface{}) {
	if logger.IsLevelEnabled(WARNING) {
		logger.write(WARNING, fmt.Sprint(args...))
	}
}

// Error prints message with error level if allowed.
func (logger *Logger) Error(args ...interface{}) {
	if logger.IsLevelEnabled(ERROR) {
		logger.write(ERROR, fmt.Sprint(args...))
	}
}

// Critical prints message with critical level if allowed.
func (logger *Logger) Critical(args ...interface{}) {
	if logger.IsLevelEnabled(CRITICAL) {
		logger.write(CRITICAL, fmt.Sprint(args...))
	}
}

// Trace prints formatted message with trace level if allowed.
func (logger *Logger) Trace(name, stage string, content interface{}) {
	logger.writeTrace(name, stage, content)
}

// Debugf prints formatted message with debug level if allowed.
func (logger *Logger) Debugf(format string, args ...interface{}) {
	if logger.IsLevelEnabled(DEBUG) {
		logger.write(DEBUG, fmt.Sprintf(format, args...))
	}
}

// Infof prints formatted message with info level if allowed.
func (logger *Logger) Infof(format string, args ...interface{}) {
	if logger.IsLevelEnabled(INFO) {
		logger.write(INFO, fmt.Sprintf(format, args...))
	}
}

// Noticef prints formatted message with notice level if allowed.
func (logger *Logger) Noticef(format string, args ...interface{}) {
	if logger.IsLevelEnabled(NOTICE) {
		logger.write(NOTICE, fmt.Sprintf(format, args...))
	}
}

// Warningf prints formatted message with warning level if allowed.
func (logger *Logger) Warningf(format string, args ...interface{}) {
	if logger.IsLevelEnabled(WARNING) {
		logger.write(WARNING, fmt.Sprintf(format, args...))
	}
}

// Errorf prints formatted message with error level if allowed.
func (logger *Logger) Errorf(format string, args ...interface{}) {
	if logger.IsLevelEnabled(ERROR) {
		logger.write(ERROR, fmt.Sprintf(format, args...))
	}
}

// Criticalf prints formatted message with critical level if allowed.
func (logger *Logger) Criticalf(format string, args ...interface{}) {
	if logger.IsLevelEnabled(CRITICAL) {
		logger.write(CRITICAL, fmt.Sprintf(format, args...))
	}
}
