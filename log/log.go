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

func (b Badger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Debugw(msg)
}

func (b Badger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Errorw(msg)
}

func (b Badger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf("[badger] "+format, args...)
	if msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	sugar.Infow(msg)
}

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
