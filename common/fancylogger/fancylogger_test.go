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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var log = NewLogger("Fancylogger unit test", DEBUG)

func TestLogger_IsLevelEnabled(t *testing.T) {
	test := struct {
		expected bool
		log      *Logger
		l        Level
	}{
		true,
		NewLogger("Fancylogger unit test", INFO),
		ERROR,
	}

	assert.Equal(t, test.expected, test.log.IsLevelEnabled(test.l))
}

func TestLogger_IsEnableCaller(t *testing.T) {
	test := struct {
		expected bool
		log      *Logger
		l        Level
	}{
		true,
		NewLogger("Fancylogger unit test", INFO),
		ERROR,
	}

	assert.Equal(t, false, test.log.enableCaller)
	test.log.SetEnableCaller(true)
	assert.Equal(t, test.expected, test.log.enableCaller)
}

func TestLogger_SetAndGet(t *testing.T) {
	//buf := &bytes.Buffer{}
	formatter := &JSONFormatter{"", true, FieldMap{}, false}
	file, err := os.Create("./fancylogger_test.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test.txt")
	}()
	assert.Nil(t, err)

	// test SetBackends()
	log.SetBackends(NewIOBackend(formatter, file))
	assert.Equal(t, NewIOBackend(formatter, file), log.Backends)

	// test SetHooks()
	log.SetHooks(NewLevelHooks())
	assert.Equal(t, NewLevelHooks(), log.Hooks)

	// test IsEnableCaller()
	assert.Equal(t, log.enableCaller, log.IsEnableCaller())

	// SetLevel() and GetLevel()
	log.SetLevel(DEBUG)
	assert.Equal(t, DEBUG, log.GetLevel())

	// SetModule() and GetModule()
	log.SetModule("Fancylogger unit test")
	assert.Equal(t, "Fancylogger unit test", log.GetModule())

	// SetEnableCaller()
	log.SetEnableCaller(false)
	assert.Equal(t, false, log.enableCaller)

	// AddCallDepth()
	log.AddCallDepth(2)
	assert.Equal(t, 2, log.callDepth)
}

func TestLogger_Info(t *testing.T) {
	file, err := os.Create("./fancylogger_test_INFO.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_INFO.txt")
	}()
	assert.Nil(t, err)

	// Test for hook=nil line:212
	log.Hooks = nil

	// Test for enableCaller line:180
	log.enableCaller = true

	setFileBackendsWithJSONFormatter(file)
	log.Info("This is an INFO message.")
	_ = file.Close()

	expected := map[string]interface{}{
		"func":   GetCallInfo(log.callDepth + 1).TrimmedPath(),
		"level":  "INFO",
		"module": "Fancylogger unit test",
		"msg":    "This is an INFO message.",
	}

	b := &bytes.Buffer{}
	encoder := json.NewEncoder(b)
	ecerr := encoder.Encode(expected)
	assert.Nil(t, ecerr)

	fR, err := os.OpenFile("./fancylogger_test_INFO.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)

	// Reset log settings
	log.enableCaller = false
	log.Hooks = DefaultHook
}

func TestLogger_Debug(t *testing.T) {
	file, err := os.Create("./fancylogger_test_DEBUG.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_DEBUG.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	//log.Backends=nil
	log.Debug("This is a DEBUG message.")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "DEBUG",
		"module": "Fancylogger unit test",
		"msg":    "This is a DEBUG message.",
	}

	fR, err := os.OpenFile("./fancylogger_test_DEBUG.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Notice(t *testing.T) {
	file, err := os.Create("./fancylogger_test_NOTICE.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_NOTICE.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Notice("This is a NOTICE message.")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "NOTICE",
		"module": "Fancylogger unit test",
		"msg":    "This is a NOTICE message.",
	}

	fR, err := os.OpenFile("./fancylogger_test_NOTICE.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Warning(t *testing.T) {
	file, err := os.Create("./fancylogger_test_WARNING.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_WARNING.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Warning("This is a WARNING message.")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "WARNING",
		"module": "Fancylogger unit test",
		"msg":    "This is a WARNING message.",
	}

	fR, err := os.OpenFile("./fancylogger_test_WARNING.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Error(t *testing.T) {
	file, err := os.Create("./fancylogger_test_ERROR.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_ERROR.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Error("This is an ERROR message.")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "ERROR",
		"module": "Fancylogger unit test",
		"msg":    "This is an ERROR message.",
	}

	fR, err := os.OpenFile("./fancylogger_test_ERROR.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Critical(t *testing.T) {
	file, err := os.Create("./fancylogger_test_CRITICAL.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_CRITICAL.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Critical("This is a CRITICAL message.")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "CRITICAL",
		"module": "Fancylogger unit test",
		"msg":    "This is a CRITICAL message.",
	}

	fR, err := os.OpenFile("./fancylogger_test_CRITICAL.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Debugf(t *testing.T) {
	file, err := os.Create("./fancylogger_test_DEBUGF.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_DEBUGF.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Debugf("This is a %s message(with format).", "DEBUG")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "DEBUG",
		"module": "Fancylogger unit test",
		"msg":    "This is a DEBUG message(with format).",
	}

	fR, err := os.OpenFile("./fancylogger_test_DEBUGF.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Infof(t *testing.T) {
	file, err := os.Create("./fancylogger_test_INFOF.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_INFOF.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Infof("This is a %s message(with format).", "INFO")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "INFO",
		"module": "Fancylogger unit test",
		"msg":    "This is a INFO message(with format).",
	}

	fR, err := os.OpenFile("./fancylogger_test_INFOF.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Noticef(t *testing.T) {
	file, err := os.Create("./fancylogger_test_NOTICEF.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_NOTICEF.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Noticef("This is a %s message(with format).", "NOTICE")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "NOTICE",
		"module": "Fancylogger unit test",
		"msg":    "This is a NOTICE message(with format).",
	}

	fR, err := os.OpenFile("./fancylogger_test_NOTICEF.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}
func TestLogger_Warningf(t *testing.T) {
	file, err := os.Create("./fancylogger_test_WARNINGF.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_WARNINGF.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Warningf("This is a %s message(with format).", "WARNING")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "WARNING",
		"module": "Fancylogger unit test",
		"msg":    "This is a WARNING message(with format).",
	}

	fR, err := os.OpenFile("./fancylogger_test_WARNINGF.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Errorf(t *testing.T) {
	file, err := os.Create("./fancylogger_test_ERRORF.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_ERRORF.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Errorf("This is a %s message(with format).", "ERROR")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "ERROR",
		"module": "Fancylogger unit test",
		"msg":    "This is a ERROR message(with format).",
	}

	fR, err := os.OpenFile("./fancylogger_test_ERRORF.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func TestLogger_Criticalf(t *testing.T) {
	file, err := os.Create("./fancylogger_test_CRITICALF.txt")
	// nolint:errcheck
	defer func() {
		_ = os.Remove("./fancylogger_test_CRITICALF.txt")
	}()

	assert.Nil(t, err)

	setFileBackendsWithJSONFormatter(file)

	log.Criticalf("This is a %s message(with format).", "CRITICALF")
	_ = file.Close()

	expected := map[string]interface{}{
		"level":  "CRITICAL",
		"module": "Fancylogger unit test",
		"msg":    "This is a CRITICALF message(with format).",
	}

	fR, err := os.OpenFile("./fancylogger_test_CRITICALF.txt", os.O_RDONLY, 777)
	// nolint:errcheck
	defer func() {
		_ = fR.Close()
	}()
	assert.Nil(t, err)
	IsEqual(t, expected, fR)
}

func IsEqual(t *testing.T, expected interface{}, file *os.File) {

	b := &bytes.Buffer{}
	encoder := json.NewEncoder(b)
	err := encoder.Encode(expected)
	assert.Nil(t, err)

	fileinfo, err := file.Stat()
	assert.Nil(t, err)

	fileSize := fileinfo.Size()
	bf := make([]byte, fileSize)
	_, err = file.Read(bf)
	assert.Nil(t, err)
	assert.Equal(t, b.Bytes(), bf)
}

func setFileBackendsWithJSONFormatter(file *os.File) {
	formatter := &JSONFormatter{"", true, FieldMap{}, false}
	log.SetBackends(NewIOBackend(formatter, file))
}
