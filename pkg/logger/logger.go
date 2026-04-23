package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cronicle/cronicle-dealer/internal/config"
)

var (
		Log *zap.Logger
)

var (
	Debug = func(msg string, fields ...zap.Field) {
		if Log != nil {
			Log.Debug(msg, fields...)
		}
	}
	Info = func(msg string, fields ...zap.Field) {
		if Log != nil {
			Log.Info(msg, fields...)
		}
	}
	Warn = func(msg string, fields ...zap.Field) {
		if Log != nil {
			Log.Warn(msg, fields...)
		}
	}
	Error = func(msg string, fields ...zap.Field) {
		if Log != nil {
			Log.Error(msg, fields...)
		}
	}
	Fatal = func(msg string, fields ...zap.Field) {
		if Log != nil {
			Log.Fatal(msg, fields...)
		}
	}
)

func InitLogger(cfg *config.LoggingConfig) error {
	level := parseLevel(cfg.Level)
	encoderCfg := newEncoderConfig(cfg.Format)
	encoder := newEncoder(cfg.Format, encoderCfg)
	writeSyncer := newWriteSyncer(cfg.Output)

	core := zapcore.NewCore(encoder, writeSyncer, level)
	Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return nil
}

func WrapCore(wrapper func(zapcore.Core) zapcore.Core) {
	if Log == nil {
		return
	}
	Log = zap.New(wrapper(Log.Core()), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func newEncoderConfig(format string) zapcore.EncoderConfig {
	if format == "json" {
		cfg := zap.NewProductionEncoderConfig()
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		return cfg
	}
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg
}

func newEncoder(format string, cfg zapcore.EncoderConfig) zapcore.Encoder {
	if format == "json" {
		return zapcore.NewJSONEncoder(cfg)
	}
	return zapcore.NewConsoleEncoder(cfg)
}

func newWriteSyncer(output string) zapcore.WriteSyncer {
	if output == "stdout" {
		return zapcore.AddSync(os.Stdout)
	}

	file, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return zapcore.AddSync(os.Stdout)
	}
	return zapcore.AddSync(file)
}

func Sync() error {
	if Log == nil {
		return nil
	}
	return Log.Sync()
}
