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

package txpool

// Logger is the TxPool logger interface which managers logger output.
type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})

	Notice(v ...interface{})
	Noticef(format string, v ...interface{})

	Warning(v ...interface{})
	Warningf(format string, v ...interface{})

	Error(v ...interface{})
	Errorf(format string, v ...interface{})

	Critical(v ...interface{})
	Criticalf(format string, v ...interface{})
}
