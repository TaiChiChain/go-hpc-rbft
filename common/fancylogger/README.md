FancyLogger
======

> Logger Service Provider interface in go.


## Table of Contents

- [Features](#Features)
- [Usage](#Usage)
- [API](#API)
- [Example](#Example)
- [Mockgen](#Mockgen)
- [GitCZ](#GitCZ)
- [Related Efforts](#Related Efforts)
- [Contribute](#Contribute)
- [License](#License)

## Features
- structured logging output with two built-in formatter: string formatter and json formatter
- custom hook reminder function
- support multiple backends, each backend can define its own output formatter
- support multiple modules with different levels
- nicely color-coded in TTY

## Usage
FancyLogger is a high-performance, structured, leveled logger service written in 
GO(golang), which is completely API compatible with the standard library logger.

If you are developing a large project with multiple independent modules, fancyLogger 
has provided a `GlobalLogger` instance which helps you manage multiple modules with 
their independent logging levels, backends even formatters. Of course, you can manage 
you sub-modules by yourself, but we recommend you to use `GlobalLogger` as `GlobalLogger` 
can help you save the trouble of duplicate default configuration form each sub-module.

If you are develop a pocket application which may only contain one module, you can 
directly create a `fancyLogger` instance with you designed module name and log level 
using `NewLogger()` method.

For test use only, you can create a `fancyLogger` instance using `NewRawLogger()` method 
which will return a Logger with string formatter and print messages to Stdout. The 
default log level is `Debug` and you can modify it using `SetLevel()` method.

### Structured output
Each `fancyLogger` instance must provide a `Backend` to output logging messages, and 
usually each `Backend` needs a `Formatter` to format messages. There are two built-in 
`Formatter`(StringFormatter and JSONFormatter) and one built-in `Backend` ioBackend 
which prints messages to a given io.Writer stream.

With StringFormatter, your output may look like:
```text
time="21:18:43.945" module=consensus level=ERROR msg="output to multi backend!!!" func="example/example.go:109" algo=RBFT replica=1
time="21:18:43.945" module=consensus level=CRITICAL msg="state update." func="example/example.go:116" algo=RBFT replica=1
```
With JSONFormatter, your output may look like:
```json
[{
   "algo": "RBFT",
   "func": "example/example.go:109",
   "level": "ERROR",
   "module": "consensus",
   "msg": "output to multi backend!!!",
   "replica": 1,
   "time": "2019-08-07T21:18:43+08:00"
 },
 {
   "algo": "RBFT",
   "func": "example/example.go:116",
   "level": "CRITICAL",
   "module": "consensus",
   "msg": "state update.",
   "replica": 1,
   "time": "2019-08-07T21:18:43+08:00"
 }]
```

### Hook reminder
FancyLogger supports custom hook reminder function which monitors each logger message and 
triggers actions according to user-defined `Hook` functions using `SetHooks(hooks ...Hook)` 
in which you can specify multiple hooks for one fancyLogger instance. Usually, you can use 
hook reminders together with application monitor tools such as logstash and Splunk.

Now, there is one built-in `Hook` --- leveledHook which triggers different actions according 
to different logging message levels.

### Backend
Backend is where logging messages will be printed to, you can set or change your logger's backend 
using `SetBackends(backends ...Backend)` whenever necessary.

One built-in `Backend` --- ioBackend is provided in fancyLogger in which you must deliver a 
Formatter(to format you logging messages) and an io.Writer(to print logging messages).

## API
FancyLogger is completely API compatible with the standard library logger. There are 6 logging 
levels from high to low:
```text
CRITICAL
ERROR
WARNING
NOTICE
INFO
DEBUG
```
You can use `Debug(args ...interface{})` to print messages or use 
`Debugf(format string, args ...interface{})` to print messages formatted with standard `fmt.Sprintf()`.

## Example
Below is a simple use case.
```go
package main

import (
	"fmt"
	"os"

	fLogger "github.com/hyperchain/go-hpc-consensus/fancylogger"
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
```

### Fields
You can specify one key-value pair as stable fields for fancyLogger using 
`logger.WithField(key string, value interface{})`, after which , each message 
printed by this logger instance will carry out this field. Optionally, you 
can use `logger.WithFields(fields Fields)` to specify multiple fields once.

## Mockgen

Install **mockgen** : `go get github.com/golang/mock/mockgen`

How to use?

- source： 指定接口文件
- destination: 生成的文件名
- package:生成文件的包名
- imports: 依赖的需要import的包
- aux_files:接口文件不止一个文件时附加文件
- build_flags: 传递给build工具的参数

Eg.`mockgen -destination mock/mock_logger.go -package logger -source logger.go`

## GitCZ

**Note**: Please use command `npm install` if you are the first time to use `git cz` in this repo.

## Related Efforts
There are also some excellent repository implementing logger service in GO where we 
have learn from.
- [logrus](https://github.com/Sirupsen/logrus) - a structured logger for Go (golang).
- [go-logging](https://github.com/op/go-logging) - a logging infrastructure for Go..
- [zap](https://github.com/uber-go/zap) - Blazing fast, structured, leveled logging in Go..
- [log15](https://github.com/ethereum/go-ethereum/tree/master/log) - an opinionated, simple toolkit for best-practice logging in Go (golang) that is both human and machine readable..

## Contribute

PRs are welcome!

Small note: If editing the Readme, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

LGPL © Kejie Zhang