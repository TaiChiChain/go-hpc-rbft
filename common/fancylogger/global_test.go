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

func TestGlobalLogger_NewModuleLogger(t *testing.T) {
	gl := NewGlobalLogger("global_test")
	mLogger := gl.NewModuleLogger("fl1", INFO)
	assert.Equal(t, DefaultBackend, mLogger.Backends)
	assert.Equal(t, DefaultBackend, mLogger.TraceBackends)

	//Test gl.Backends!=nil
	gl.SetBackends(DefaultBackend)
	mLogger = gl.NewModuleLogger("fl1", INFO)
	assert.Equal(t, DefaultBackend, mLogger.Backends)
	assert.Equal(t, DefaultBackend, mLogger.TraceBackends)
}

func TestGlobalLogger_SetAndGet(t *testing.T) {
	gl := NewGlobalLogger("global_test")
	gl.SetBackends(DefaultBackend)
	gl.SetHooks(DefaultHook)
	gl.moduleLoggers["m1"] = NewLogger("fl1", DEBUG)

	assert.Equal(t, DEBUG, gl.GetModuleLevel("m1"))
	assert.Equal(t, CRITICAL, gl.GetModuleLevel("not_ok"))

	gl.SetModuleLevel("m1", INFO)
	assert.Equal(t, INFO, gl.GetModuleLevel("m1"))

	gl.SetModuleLevel("not_ok", INFO)
	assert.Equal(t, INFO, gl.GetModuleLevel("m1"))

	assert.Equal(t, true, gl.IsModuleLevelEnabled("m1", ERROR))
	assert.Equal(t, false, gl.IsModuleLevelEnabled("not_ok", ERROR))

	assert.Equal(t, false, gl.IsEnableCaller())
	gl.SetEnableCaller(true)
	assert.Equal(t, true, gl.IsEnableCaller())

	gl.SetBackends(NewNopBackend())
	assert.Equal(t, NewNopBackend(), gl.Backends)
}
