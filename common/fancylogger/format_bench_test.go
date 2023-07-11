package fancylogger

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

var smallFields = Fields{
	"foo":   "bar",
	"baz":   "qux",
	"one":   "two",
	"three": "four",
}

var largeFields = Fields{
	"foo":       "bar",
	"baz":       "qux",
	"one":       "two",
	"three":     "four",
	"five":      "six",
	"seven":     "eight",
	"nine":      "ten",
	"eleven":    "twelve",
	"thirteen":  "fourteen",
	"fifteen":   "sixteen",
	"seventeen": "eighteen",
	"nineteen":  "twenty",
	"a":         "b",
	"c":         "d",
	"e":         "f",
	"g":         "h",
	"i":         "j",
	"k":         "l",
	"m":         "n",
	"o":         "p",
	"q":         "r",
	"s":         "t",
	"u":         "v",
	"w":         "x",
	"y":         "z",
	"this":      "will",
	"make":      "thirty",
	"entries":   "yeah",
}

var errorFields = Fields{
	"foo": fmt.Errorf("bar"),
	"baz": fmt.Errorf("qux"),
}

func BenchmarkErrorJSONFormatter_Format(b *testing.B) {
	doBenchmark(b, &JSONFormatter{"", true, FieldMap{}, false}, errorFields)
}

func BenchmarkSmallJSONFormatter_Format(b *testing.B) {
	doBenchmark(b, &JSONFormatter{"", true, FieldMap{}, false}, smallFields)
}

func BenchmarkLargeJSONFormatter_Format(b *testing.B) {
	doBenchmark(b, &JSONFormatter{"", true, FieldMap{}, false}, largeFields)
}

func BenchmarkSmallColoredStringFormatter_Format(b *testing.B) {
	doBenchmark(b, &StringFormatter{true, true,
		"", false, true,
		true, true, FieldMap{}}, smallFields)
}

func BenchmarkErrorColoredStringFormatter_Format(b *testing.B) {
	doBenchmark(b, &StringFormatter{true, true,
		"", false, true,
		true, true, FieldMap{}}, errorFields)
}

func BenchmarkLargeColoredStringFormatter_Format(b *testing.B) {
	doBenchmark(b, &StringFormatter{true, true,
		"", false, true,
		true, true, FieldMap{}}, largeFields)
}

func doBenchmark(b *testing.B, formatter Formatter, fields Fields) {
	var callerInfo CallerInfo

	record := &Record{
		Module:       "Benchmark Test",
		Level:        ERROR,
		Fields:       fields,
		Time:         time.Now(),
		EnableCaller: false,
		Caller:       callerInfo,
		Message:      "the benchmark message",
		Buffer:       bytes.NewBuffer(nil),
	}

	for i := 0; i < b.N; i++ {
		if _, err := formatter.Format(record); err != nil {
			b.Fatal(err)
		}
	}
}
