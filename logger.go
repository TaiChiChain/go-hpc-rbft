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

package rbft

// Logger is the RBFT logger interface which managers logger output.
type Logger interface {
	Debug(v ...any)
	Debugf(format string, v ...any)

	Info(v ...any)
	Infof(format string, v ...any)

	Notice(v ...any)
	Noticef(format string, v ...any)

	Warning(v ...any)
	Warningf(format string, v ...any)

	Error(v ...any)
	Errorf(format string, v ...any)

	Critical(v ...any)
	Criticalf(format string, v ...any)

	Trace(name, stage string, content any)
}
