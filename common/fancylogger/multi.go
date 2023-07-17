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

type multiBackends struct {
	backends []Backend
}

// MultiBackends returns a multiBackends composite by given backends.
func MultiBackends(backends ...Backend) Backend {
	if len(backends) == 0 {
		return NewNopBackend()
	}
	var bs []Backend
	for _, b := range backends {
		bs = append(bs, b)
	}
	return &multiBackends{
		backends: bs,
	}
}

// Write implements the Backend interface.
func (mbs *multiBackends) Write(record *Record) error {
	for i := range mbs.backends {
		if err := mbs.backends[i].Write(record); err != nil {
			return err
		}
	}
	return nil
}

type multiHooks struct {
	mHooks []Hook
}

// MultiHooks returns a multiHooks composite by given hooks.
func MultiHooks(hooks ...Hook) Hook {
	if len(hooks) == 0 {
		return NewNopHook()
	}

	var mh []Hook
	for _, hook := range hooks {
		mh = append(mh, hook)
	}
	return &multiHooks{
		mHooks: mh,
	}
}

// Fire implements the Hook interface.
func (mh *multiHooks) Fire(record *Record) error {
	for _, hook := range mh.mHooks {
		if err := hook.Fire(record); err != nil {
			return err
		}
	}
	return nil
}
