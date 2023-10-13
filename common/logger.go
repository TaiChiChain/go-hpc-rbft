package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

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

type SimpleLogger struct {
	logger logrus.FieldLogger
	prefix string
}

func NewSimpleLogger() *SimpleLogger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			_, filename := filepath.Split(f.File)
			return "", fmt.Sprintf("%12s:%-4d", filename, f.Line)
		},
	})
	logger.SetReportCaller(false)
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.DebugLevel)
	return &SimpleLogger{
		logger: logger,
	}
}

func (lg *SimpleLogger) SetPrefix(prefix string) {
	lg.prefix = prefix
}

func (lg *SimpleLogger) Debug(v ...any) {
	lg.logger.Debug(append([]any{lg.prefix}, v...)...)
}

func (lg *SimpleLogger) Debugf(format string, v ...any) {
	lg.logger.Debugf(lg.prefix+format, v...)
}

func (lg *SimpleLogger) Info(v ...any) {
	lg.logger.Info(append([]any{lg.prefix}, v...)...)
}

func (lg *SimpleLogger) Infof(format string, v ...any) {
	lg.logger.Infof(lg.prefix+format, v...)
}

func (lg *SimpleLogger) Warning(v ...any) {
	lg.logger.Warning(append([]any{lg.prefix}, v...)...)
}

func (lg *SimpleLogger) Warningf(format string, v ...any) {
	lg.logger.Warningf(lg.prefix+format, v...)
}

func (lg *SimpleLogger) Error(v ...any) {
	lg.logger.Error(append([]any{lg.prefix}, v...)...)
}

func (lg *SimpleLogger) Errorf(format string, v ...any) {
	lg.logger.Errorf(lg.prefix+format, v...)
}

func (lg *SimpleLogger) Critical(v ...any) {
	lg.logger.Error(append([]any{lg.prefix}, v...)...)
}

func (lg *SimpleLogger) Criticalf(format string, v ...any) {
	lg.logger.Errorf(lg.prefix+format, v...)
}

// Trace implements rbft.Logger.
func (lg *SimpleLogger) Trace(name string, stage string, content any) {
	lg.Info(name, stage, content)
}

func (lg *SimpleLogger) Notice(v ...any) {
	lg.Info(v...)
}

func (lg *SimpleLogger) Noticef(format string, v ...any) {
	lg.Infof(format, v...)
}
