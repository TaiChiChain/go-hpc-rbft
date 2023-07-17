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

import "sync"

// GlobalLogger is the global logger instance which helps manage multi fancyLogger.
type GlobalLogger struct {
	// The global logger name. It's only specified once when creating global logger
	// instance.
	Name string

	// default backend for globalLogger.
	Backends      Backend
	TraceBackends Backend

	// default hook functions for globalLogger.
	Hooks Hook

	// enable print caller info or not.
	enableCaller bool

	// all module logger instances
	moduleLoggers map[string]*Logger

	mu sync.Mutex
}

// NewGlobalLogger creates a GlobalLogger instance.
func NewGlobalLogger(name string) *GlobalLogger {
	return &GlobalLogger{
		Name:          name,
		moduleLoggers: make(map[string]*Logger),
	}
}

// NewModuleLogger creates a specified fancyLogger instance using GlobalLogger's Backend and Hook
// if any, module logger will be recorded in moduleLoggers.
func (gl *GlobalLogger) NewModuleLogger(module string, level Level) *Logger {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	var (
		backend      Backend
		traceBackend Backend
	)
	if gl.Backends == nil {
		backend = DefaultBackend
	} else {
		backend = gl.Backends
	}
	if gl.TraceBackends == nil {
		traceBackend = DefaultBackend
	} else {
		traceBackend = gl.TraceBackends
	}

	ml, ok := gl.moduleLoggers[module]
	if !ok {
		logger := NewLogger(module, level)
		logger.SetBackends(backend)
		logger.SetTraceBackends(traceBackend)
		if gl.Hooks != nil {
			logger.SetHooks(gl.Hooks)
		}
		logger.SetEnableCaller(gl.enableCaller)
		return logger
	}
	ml.SetLevel(level)
	return ml
}

// GetModuleLevel returns the default level of the given module's Logger.
func (gl *GlobalLogger) GetModuleLevel(module string) Level {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	ml, ok := gl.moduleLoggers[module]
	if !ok {
		return CRITICAL
	}
	return ml.GetLevel()
}

// SetModuleLevel sets the default level of the given module's Logger.
func (gl *GlobalLogger) SetModuleLevel(module string, level Level) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	ml, ok := gl.moduleLoggers[module]
	if !ok {
		return
	}
	ml.SetLevel(level)
}

// IsModuleLevelEnabled returns if the given logger level is enabled under given module's Logger.
func (gl *GlobalLogger) IsModuleLevelEnabled(module string, level Level) bool {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	ml, ok := gl.moduleLoggers[module]
	if !ok {
		return false
	}
	return ml.IsLevelEnabled(level)
}

// SetBackends sets the backend of GlobalLogger.
func (gl *GlobalLogger) SetBackends(backends ...Backend) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	gl.Backends = compositeBackend(backends...)
}

// SetTraceBackends sets the trace backend of GlobalLogger.
func (gl *GlobalLogger) SetTraceBackends(backends ...Backend) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	gl.TraceBackends = compositeBackend(backends...)
}

// SetHooks sets the hook of GlobalLogger.
func (gl *GlobalLogger) SetHooks(hooks ...Hook) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	gl.Hooks = compositeHook(hooks...)
}

// IsEnableCaller returns if GlobalLogger enables printing caller info or not.
func (gl *GlobalLogger) IsEnableCaller() bool {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	return gl.enableCaller
}

// SetEnableCaller sets enableCaller of GlobalLogger.
func (gl *GlobalLogger) SetEnableCaller(enable bool) {
	gl.mu.Lock()
	defer gl.mu.Unlock()
	gl.enableCaller = enable
}
