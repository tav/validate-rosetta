// Copyright 2021 Coinbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package log provides utility functions for logging to the console.
package log

import (
	"fmt"

	"github.com/tav/validate-rosetta/process"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger
)

// Badger wraps the global zap.Logger for the badger.Logger interface.
type Badger struct{}

// Debugf uses fmt.Sprintf to log a formatted string.
func (b Badger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Debugw(msg)
}

// Errorf uses fmt.Sprintf to log a formatted string.
func (b Badger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Errorw(msg)
}

// Infof uses fmt.Sprintf to log a formatted string.
func (b Badger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Infow(msg)
}

// Warningf uses fmt.Sprintf to log a formatted string.
func (b Badger) Warningf(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Warnw(msg)
}

// Error logs an error message with any optional fields.
func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

// Errorf uses fmt.Sprintf to log a formatted string.
func Errorf(format string, args ...interface{}) {
	sugar.Errorf(format, args...)
}

// Fatalf uses fmt.Sprintf to log a formatted string, and then calls
// process.Exit.
func Fatalf(format string, args ...interface{}) {
	sugar.Errorf(format, args...)
	process.Exit(1)
}

// Info logs an info message with any optional fields.
func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

// Infof uses fmt.Sprintf to log a formatted string.
func Infof(format string, args ...interface{}) {
	sugar.Infof(format, args...)
}

// Init initializes the global logger.
func Init() {
	enc := zap.NewDevelopmentEncoderConfig()
	enc.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg := zap.Config{
		DisableCaller:     true,
		DisableStacktrace: true,
		EncoderConfig:     enc,
		Encoding:          "console",
		ErrorOutputPaths:  []string{"stderr"},
		Level:             zap.NewAtomicLevelAt(zap.InfoLevel),
		OutputPaths:       []string{"stderr"},
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
	}
	logger, _ = cfg.Build()
	sugar = logger.Sugar()
	zap.RedirectStdLog(logger)
	process.SetExitHandler(func() {
		logger.Sync()
	})
}
