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

var (
	// DefaultHook is the default hook implement if user doesn't specify the hook.
	DefaultHook = NewNopHook()
)

// Hook is used to trigger whatever action when record satisfy the user defined condition.
type Hook interface {
	Fire(record *Record) error
}

type nopHook struct{}

// NewNopHook creates a no-op hook used for test.
func NewNopHook() Hook {
	return &nopHook{}
}

// Fire implements the Hook interface.
func (nh *nopHook) Fire(record *Record) error {
	return nil
}

// LevelHooks triggers different action when printing logs with different levels.
type LevelHooks struct {
	hookMap map[Level][]Hook
}

// NewLevelHooks creates a LevelHooks instance.
func NewLevelHooks() *LevelHooks {
	return &LevelHooks{
		hookMap: make(map[Level][]Hook),
	}
}

// Add adds hook function to given level action.
func (lh *LevelHooks) Add(hook Hook, levels ...Level) {
	if len(levels) == 0 {
		return
	}

	for _, level := range levels {
		lh.hookMap[level] = append(lh.hookMap[level], hook)
	}
}

// Fire implements the Hook interface.
func (lh *LevelHooks) Fire(record *Record) error {
	if hooks, ok := lh.hookMap[record.Level]; ok {
		for _, hook := range hooks {
			if err := hook.Fire(record); err != nil {
				return err
			}
		}
	}
	return nil
}

func compositeHook(hooks ...Hook) Hook {
	if len(hooks) == 0 {
		return DefaultHook
	}
	if len(hooks) == 1 {
		return hooks[0]
	}
	return MultiHooks(hooks...)
}
