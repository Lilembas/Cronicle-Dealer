package manager

import (
	"go.uber.org/zap/zapcore"
)

type logHookCore struct {
	zapcore.Core
	buffer *LogBuffer
}

func (c *logHookCore) With(fields []zapcore.Field) zapcore.Core {
	return &logHookCore{Core: c.Core.With(fields), buffer: c.buffer}
}

func (c *logHookCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		ce = ce.AddCore(entry, c)
	}
	return c.Core.Check(entry, ce)
}

func (c *logHookCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	fieldsJSON := "{}"
	if len(fields) > 0 {
		enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "",
			LevelKey:       "",
			NameKey:        "",
			CallerKey:      "",
			FunctionKey:    "",
			MessageKey:     "",
			StacktraceKey:  "",
			LineEnding:     "",
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
		buf, _ := enc.EncodeEntry(entry, fields)
		fieldsJSON = buf.String()
		buf.Free()
	}

	c.buffer.Write(entry, fieldsJSON)
	return c.Core.Write(entry, fields)
}
