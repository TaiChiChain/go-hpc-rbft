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

package main

import (
	"fmt"
	"os"

	fLogger "github.com/axiomesh/axiom-bft/common/fancylogger"
)

type mockHook struct {
	addr string
}

func (mh *mockHook) Fire(e *fLogger.Record) error {
	if e.Level == fLogger.CRITICAL {
		fmt.Printf("critical happened, detailed: %s, send to addr: %s\n", e.Message, mh.addr)
	} else if e.Level == fLogger.ERROR {
		fmt.Printf("error happened, detailed: %s, send to addr: %s\n", e.Message, mh.addr)
	} else {
		fmt.Printf("nothing happened\n")
	}
	return nil
}

func main() {
	// init project-level global logger with stdout backend + string formatter
	globalLogger := fLogger.NewGlobalLogger("flato")
	// init string formatter which print colored messages in terminal
	stringFormatter := &fLogger.StringFormatter{
		EnableColors:    true,
		IsTerminal:      true,
		TimestampFormat: "2006-01-02T15:04:05.000",
	}
	// set backend which prints messages to stdout
	backend := fLogger.NewIOBackend(stringFormatter, os.Stdout)
	globalLogger.SetBackends(backend)
	// enable printing caller function info
	globalLogger.SetEnableCaller(true)

	// init sub-module logger for consensus using default config derived from globalLogger
	clog := globalLogger.NewModuleLogger("consensus", fLogger.DEBUG)
	// init sub-module logger for p2p using default config derived from globalLogger
	plog := globalLogger.NewModuleLogger("p2p", fLogger.ERROR)

	// print test log
	clog.Critical("critical consensus level")
	clog.Error("error consensus level")
	clog.Warning("warning consensus level")
	clog.Notice("notice consensus level")
	clog.Info("info consensus level")
	clog.Debug("debug consensus level")

	plog.Critical("critical p2p level")
	plog.Error("error p2p level")
	plog.Warning("warning p2p level")
	plog.Notice("notice p2p level")
	plog.Info("info p2p level")
	plog.Debug("debug p2p level")

	// add custom key
	_ = clog.WithField("replica", 1)
	clog.Noticef("received null request from replica %d", 2)

	_ = clog.WithField("algo", "RAFT")
	clog.Critical("init algo to RAFT")

	// switch consensus algorithm
	_ = clog.WithField("algo", "RBFT")
	clog.Critical("change algo to RBFT")

	// test file backend + string formatter
	_ = os.Remove("./consensus.log")
	file, _ := os.OpenFile("./consensus.log", os.O_CREATE|os.O_WRONLY, 0666)
	stringFormatter1 := &fLogger.StringFormatter{
		TimestampFormat: "15:04:05.000",
	}
	backend1 := fLogger.NewIOBackend(stringFormatter1, file)

	// switch logger backend to from stdout to file
	clog.SetBackends(backend1)
	clog.Critical("Write log to file with string format")

	// test file backend + json formatter
	_ = os.Remove("./p2p.log")
	file2, _ := os.OpenFile("./p2p.log", os.O_CREATE|os.O_WRONLY, 0666)
	jsonFormatter2 := &fLogger.JSONFormatter{
		PrettyPrint: true,
	}
	backend2 := fLogger.NewIOBackend(jsonFormatter2, file2)

	// switch p2p logger backend to file
	plog.SetBackends(backend2)
	plog.Critical("Write log to file with json format")

	// test multi backend
	multiBackend := fLogger.MultiBackends(backend, backend1, backend2)
	clog.SetBackends(multiBackend)
	clog.Error("output to multi backend!!!")

	// test hook method
	mh := &mockHook{addr: "172.16.1.1"}
	lhook := fLogger.NewLevelHooks()
	lhook.Add(mh, fLogger.CRITICAL, fLogger.ERROR)
	clog.SetHooks(lhook)
	clog.Critical("state update.")
}
