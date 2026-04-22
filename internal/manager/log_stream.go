package manager

import (
	"time"

	"go.uber.org/zap/zapcore"
)

var fieldsEncoder = zapcore.NewJSONEncoder(zapcore.EncoderConfig{
	EncodeLevel:    zapcore.LowercaseLevelEncoder,
	EncodeTime:     zapcore.ISO8601TimeEncoder,
	EncodeDuration: zapcore.SecondsDurationEncoder,
	EncodeCaller:   zapcore.ShortCallerEncoder,
})

type logHookCore struct {
	zapcore.Core
	buffer *LogBuffer
}

func (c *logHookCore) With(fields []zapcore.Field) zapcore.Core {
	return &logHookCore{Core: c.Core.With(fields), buffer: c.buffer}
}

func (c *logHookCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		if ce == nil {
			ce = &zapcore.CheckedEntry{}
		}
		ce = ce.AddCore(entry, c)
	}
	return ce
}

func (c *logHookCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}

	fieldsJSON := "{}"
	if len(fields) > 0 {
		buf, _ := fieldsEncoder.EncodeEntry(entry, fields)
		fieldsJSON = buf.String()
		buf.Free()
	}

	c.buffer.Write(entry, fieldsJSON)
	return c.Core.Write(entry, fields)
}
