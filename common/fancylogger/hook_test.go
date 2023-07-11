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
	"testing"
	"time"

	"github.com/hyperchain/go-hpc-rbft/v2/common/consensus"

	"github.com/stretchr/testify/assert"
)

type TestHook struct {
	Fired bool
	buf   *bytes.Buffer
}

func (hook *TestHook) Fire(r *Record) error {
	hook.Fired = true
	hook.buf.WriteString(r.Level.String())
	r.Buffer.WriteString(r.Level.String())
	return nil
}

func TestLevelHooks_Add(t *testing.T) {
	lh := &LevelHooks{make(map[Level][]Hook)}
	lh.Add(NewNopHook())
	assert.Equal(t, &LevelHooks{make(map[Level][]Hook)}, lh)

	lh.Add(NewNopHook(), INFO)
	expected := &LevelHooks{make(map[Level][]Hook)}
	expected.hookMap[INFO] = append(expected.hookMap[INFO], NewNopHook())

	assert.Equal(t, expected, lh)
}

func TestNewHooks(t *testing.T) {
	eNop := &nopHook{}
	eLevel := &LevelHooks{make(map[Level][]Hook)}

	nh := NewNopHook()
	lh := NewLevelHooks()

	assert.Equal(t, eNop, nh)
	assert.Equal(t, eLevel, lh)
}

func TestNopHook_Fire(t *testing.T) {
	assert.Nil(t, NewNopHook().Fire(nil))
}

func TestLevelHooks_Fire(t *testing.T) {
	hook := &TestHook{false,
		new(bytes.Buffer)}
	lh := &LevelHooks{make(map[Level][]Hook)}
	record := &Record{
		Module:       "test",
		Level:        ERROR,
		Fields:       make(map[string]interface{}),
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       CallerInfo{},
		Message:      "a new message",
		Buffer:       bytes.NewBuffer(nil),
	}

	lh.hookMap[record.Level] = append(lh.hookMap[record.Level], hook)
	_ = lh.Fire(record)

	assert.Equal(t, true, hook.Fired)
	assert.Equal(t, record.Buffer, hook.buf)
}

func TestCompositeHook(t *testing.T) {
	hooks := []Hook{
		NewNopHook(),
		NewLevelHooks(),
		&TestHook{false,
			new(bytes.Buffer)},
	}

	assert.Equal(t, DefaultHook, compositeHook())
	assert.Equal(t, hooks[0], compositeHook(hooks[0]))
	assert.Equal(t, MultiHooks(hooks...), compositeHook(hooks...))
}

type TagStageHook struct {
	stageMap map[string]string
}

func NewTagHook() *TagStageHook {
	return &TagStageHook{stageMap: make(map[string]string)}
}

func (th *TagStageHook) AddHook(stage, addr string) {
	th.stageMap[stage] = addr
}

func (th *TagStageHook) Fire(record *Record) error {
	for hookStage, addr := range th.stageMap {
		record.Tags[consensus.TagStageKey] = hookStage
		// trigger hooks for specified tag stage to addr such as MQ.
		fmt.Printf("send tag %+v to MQ server %s\n", record.Tags, addr)
	}
	return nil
}

type LeveledHook struct {
	hookMap map[Level]string
}

func NewLeveledHook() *LeveledHook {
	return &LeveledHook{hookMap: make(map[Level]string)}
}

func (lh *LeveledHook) AddHook(level Level, addr string) {
	lh.hookMap[level] = addr
}

func (lh *LeveledHook) Fire(record *Record) error {
	for hookLevel, addr := range lh.hookMap {
		if record.Level == hookLevel {
			// trigger tags for specified level such as MQ.
			fmt.Printf("send level %s to MQ server %s\n", hookLevel, addr)

		}
	}
	return nil
}
