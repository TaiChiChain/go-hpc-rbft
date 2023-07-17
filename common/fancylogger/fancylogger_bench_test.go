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
	"io"
	"os"
	"testing"
)

var loggerFields = Fields{
	"foo":   "bar",
	"baz":   "qux",
	"one":   "two",
	"three": "four",
}

func BenchmarkLoggerJSONFormatter(b *testing.B) {
	nullf, err := os.OpenFile("/dev/null", os.O_WRONLY, 0666)

	if err != nil {
		b.Fatalf("%v", err)
	}
	// nolint:errcheck
	defer func() {
		_ = nullf.Close()
	}()
	doLoggerBenchmarkWithFormatter(b, &JSONFormatter{"", true, FieldMap{}, false}, nullf, loggerFields)
}

func BenchmarkLoggerStringFormatter(b *testing.B) {
	nullf, err := os.OpenFile("/dev/null", os.O_WRONLY, 0666)

	if err != nil {
		b.Fatalf("%v", err)
	}
	// nolint:errcheck
	defer func() {
		_ = nullf.Close()
	}()
	doLoggerBenchmarkWithFormatter(b, &StringFormatter{true, true,
		"", false, true,
		true, true, FieldMap{}}, nullf, loggerFields)

}

func doLoggerBenchmarkWithFormatter(b *testing.B, f Formatter, out io.Writer, fields Fields) {
	b.SetParallelism(100)
	log := NewLogger("Fancylogger_bench_test", INFO)

	log.SetBackends(NewIOBackend(f, out))
	_ = log.WithFields(fields)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			log.Info("this is a fancylogger benchmark test")
		}
	})
}
