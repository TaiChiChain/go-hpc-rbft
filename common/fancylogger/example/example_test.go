package main

import (
	"bytes"
	"testing"
	"time"

	fLogger "github.com/axiomesh/axiom-bft/common/fancylogger"

	"github.com/stretchr/testify/assert"
)

func TestMockHook_Fire(t *testing.T) {
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	var callerInfo fLogger.CallerInfo

	record1 := &fLogger.Record{
		Module:       "test",
		Level:        fLogger.CRITICAL,
		Fields:       fields,
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       callerInfo,
		Message:      "a test message",
		Buffer:       bytes.NewBuffer(nil),
	}
	record2 := &fLogger.Record{
		Module:       "test",
		Level:        fLogger.ERROR,
		Fields:       fields,
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       callerInfo,
		Message:      "a test message",
		Buffer:       bytes.NewBuffer(nil),
	}
	record3 := &fLogger.Record{
		Module:       "test",
		Level:        fLogger.WARNING, //equal to NOTICE,INFO,DEBUG
		Fields:       fields,
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       callerInfo,
		Message:      "a test message",
		Buffer:       bytes.NewBuffer(nil),
	}

	test1 := struct {
		expected error
		record   *fLogger.Record
	}{
		nil,
		record1,
	}
	test2 := struct {
		expected error
		record   *fLogger.Record
	}{
		nil,
		record2,
	}
	test3 := struct {
		expected error
		record   *fLogger.Record
	}{
		nil,
		record3,
	}

	var mh mockHook
	assert.Equal(t, test1.expected, mh.Fire(test1.record))
	assert.Equal(t, test2.expected, mh.Fire(test2.record))
	assert.Equal(t, test3.expected, mh.Fire(test3.record))
}
