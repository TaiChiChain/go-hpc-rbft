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
	"fmt"
	"runtime"
	"sort"
	"strings"
)

type color int

const (
	// ColorBlack is the color code of black shown on terminal.
	ColorBlack = iota + 30
	// ColorRed is the color code of red shown on terminal.
	ColorRed
	// ColorGreen is the color code of green shown on terminal.
	ColorGreen
	// ColorYellow is the color code of yellow shown on terminal.
	ColorYellow
	// ColorBlue is the color code of blue shown on terminal.
	ColorBlue
	// ColorMagenta is the color code of magenta shown on terminal.
	ColorMagenta
	// ColorCyan is the color code of cyan shown on terminal.
	ColorCyan
	// ColorWhite is the color code of white shown on terminal.
	ColorWhite
)

var (
	colors = map[Level]string{
		TRACE:    ColorSeq(ColorBlue),
		CRITICAL: ColorSeq(ColorMagenta),
		ERROR:    ColorSeq(ColorRed),
		WARNING:  ColorSeq(ColorYellow),
		NOTICE:   ColorSeq(ColorGreen),
		INFO:     ColorSeq(ColorCyan),
		DEBUG:    ColorSeq(ColorWhite),
	}
)

// ColorSeq returns the given color prefix when printing messages in terminal.
func ColorSeq(color color) string {
	return fmt.Sprintf("\033[%dm", int(color))
}

// StringFormatter formats logs into string
type StringFormatter struct {
	// EnableColors indicates whether enable color printing or not.
	EnableColors bool

	// DisableTimestamp indicates whether disable printing timestamp or not.
	DisableTimestamp bool

	// TimestampFormat specify the output timestamp format.
	TimestampFormat string

	// DisableSorting indicates whether disable sorting keys or not.
	DisableSorting bool

	// DisableLevelTruncation indicates whether disable truncate levels or not.
	DisableLevelTruncation bool

	// IsTerminal indicates whether prints on terminal or not.
	IsTerminal bool

	// QuoteEmptyFields indicates whether quote empty fields or not.
	QuoteEmptyFields bool

	// FieldMap records the filtered key if users specified some conflict keys in Records.
	FieldMap FieldMap
}

// Format implements the Formatter interface or not.
func (sf *StringFormatter) Format(record *Record) ([]byte, error) {
	fieldsData := make(Fields, len(record.Fields))
	for k, v := range record.Fields {
		fieldsData[k] = v
	}
	prefixFieldClashes(fieldsData, sf.FieldMap, record.EnableCaller)
	fieldsKeys := make([]string, 0, len(fieldsData))
	for k := range fieldsData {
		fieldsKeys = append(fieldsKeys, k)
	}

	fixedKeys := make([]string, 0, 5+len(fieldsData))
	// TRACE level need not be written if print without color.
	if record.Level != TRACE {
		fixedKeys = append(fixedKeys, sf.FieldMap.resolve(FieldKeyLevel))
	}

	// print timestamp if needed
	timestampFormat := sf.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}
	if !sf.DisableTimestamp {
		fixedKeys = append(fixedKeys, sf.FieldMap.resolve(FieldKeyTime))
	}

	// print module name
	if record.Module != "" {
		fixedKeys = append(fixedKeys, sf.FieldMap.resolve(FieldKeyModule))
	}

	// print caller if needed
	if record.EnableCaller {
		fixedKeys = append(fixedKeys, sf.FieldMap.resolve(FieldKeyFunc))
	}

	// print message
	if record.Message != "" {
		fixedKeys = append(fixedKeys, sf.FieldMap.resolve(FieldKeyMsg))
	}

	if !sf.DisableSorting {
		sort.Strings(fieldsKeys)
		fixedKeys = append(fixedKeys, fieldsKeys...)
	} else {
		fixedKeys = append(fixedKeys, fieldsKeys...)
	}

	// tags will be printed using json format.
	tagsData := make(Fields, len(record.Tags))
	for k, v := range record.Tags {
		tagsData[k] = v
	}

	var b *bytes.Buffer
	if record.Buffer != nil {
		b = record.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	if sf.isColored() {
		pErr := sf.printColored(b, record, fieldsKeys, fieldsData, tagsData, timestampFormat)
		if pErr != nil {
			return nil, pErr
		}
	} else {
		for _, key := range fixedKeys {
			var value interface{}
			switch {
			case key == sf.FieldMap.resolve(FieldKeyTime):
				value = record.Time.Format(timestampFormat)
			case key == sf.FieldMap.resolve(FieldKeyModule):
				value = record.Module
			case key == sf.FieldMap.resolve(FieldKeyLevel):
				value = record.Level.String()
			case key == sf.FieldMap.resolve(FieldKeyMsg):
				value = record.Message
			case key == sf.FieldMap.resolve(FieldKeyFunc) && record.EnableCaller:
				value = record.Caller.TrimmedPath()
			default:
				// print field kvs.
				value = fieldsData[key]
			}
			sf.appendKeyValue(b, key, value)
		}

		// print tags using json format.
		if len(tagsData) != 0 {
			b.WriteByte(' ')
			encoder := json.NewEncoder(b)
			if err := encoder.Encode(tagsData); err != nil {
				return nil, err
			}
		} else {
			b.WriteByte('\n')
		}
	}

	return b.Bytes(), nil
}

func (sf *StringFormatter) isColored() bool {
	isColored := sf.EnableColors || (sf.IsTerminal && (runtime.GOOS != "windows"))
	return isColored
}

func (sf *StringFormatter) printColored(b *bytes.Buffer, record *Record, fieldsKeys []string,
	fieldsData, tagsData Fields, timestampFormat string) error {
	var levelColor string
	levelColor, ok := colors[record.Level]
	if !ok {
		levelColor = ColorSeq(ColorBlue)
	}

	levelText := strings.ToUpper(record.Level.String())
	if !sf.DisableLevelTruncation {
		levelText = levelText[0:4]
	}

	// print color prefix
	_, _ = fmt.Fprintf(b, "%s%s", levelColor, levelText)

	// print timestamp if needed
	if !sf.DisableTimestamp {
		_, _ = fmt.Fprintf(b, " [%s]", record.Time.Format(timestampFormat))
	}

	// print module name
	if record.Module != "" {
		_, _ = fmt.Fprintf(b, " [%s]", record.Module)
	}

	// print caller if needed
	if record.EnableCaller {
		caller := record.Caller.TrimmedPath()
		_, _ = fmt.Fprintf(b, " %s", caller)
	}

	// print message
	if record.Message != "" {
		record.Message = strings.TrimSuffix(record.Message, "\n")
		_, _ = fmt.Fprintf(b, " %s", record.Message)
	}

	// print field kvs.
	for _, k := range fieldsKeys {
		v := fieldsData[k]
		// nolint:errcheck
		_, _ = fmt.Fprintf(b, " %s=", k)
		sf.appendValue(b, v)
	}

	// print tags using json format.
	if len(tagsData) != 0 {
		b.WriteByte(' ')
		b.WriteString("tag")
		b.WriteByte('=')
		encoder := json.NewEncoder(b)
		if err := encoder.Encode(tagsData); err != nil {
			return err
		}
	} else {
		b.WriteByte('\n')
	}

	// print color suffix
	_, _ = fmt.Fprintf(b, "\033[0m")
	return nil
}

func (sf *StringFormatter) needsQuoting(text string) bool {
	if sf.QuoteEmptyFields && len(text) == 0 {
		return true
	}
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '.' || ch == '_' || ch == '/' || ch == '@' || ch == '^' || ch == '+') {
			return true
		}
	}
	return false
}

func (sf *StringFormatter) appendKeyValue(b *bytes.Buffer, key string, value interface{}) {
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	b.WriteString(key)
	b.WriteByte('=')
	sf.appendValue(b, value)
}

func (sf *StringFormatter) appendValue(b *bytes.Buffer, value interface{}) {
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}

	if !sf.needsQuoting(stringVal) {
		b.WriteString(stringVal)
	} else {
		b.WriteString(fmt.Sprintf("%q", stringVal))
	}
}
