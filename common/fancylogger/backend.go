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

import "io"

var (
	// DefaultBackend is the default backend if user doesn't specify the Backend
	DefaultBackend = NewNopBackend()
)

// Backend is the interface used to write record to actual output stream.
type Backend interface {
	Write(record *Record) error
}

type nopBackend struct{}

// NewNopBackend creates a no-op backend used for test.
func NewNopBackend() Backend {
	return &nopBackend{}
}

// Write implements the Backend interface.
func (nb *nopBackend) Write(record *Record) error {
	return nil
}

type ioBackend struct {
	formatter Formatter
	out       io.Writer
}

// NewIOBackend creates a ioBackend instance which write records to io stream.
func NewIOBackend(f Formatter, o io.Writer) Backend {
	return &ioBackend{
		formatter: f,
		out:       o,
	}
}

// Write implements the Backend interface.
func (ib *ioBackend) Write(record *Record) error {
	// TODO reuse buffer ?
	record.Buffer.Reset()
	buf, err := ib.formatter.Format(record)
	if err != nil {
		return err
	}

	_, err = ib.out.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

func compositeBackend(backends ...Backend) Backend {
	if len(backends) == 0 {
		return DefaultBackend
	}
	if len(backends) == 1 {
		return backends[0]
	}
	return MultiBackends(backends...)
}
