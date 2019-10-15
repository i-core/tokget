/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package log

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey struct{}

var ctxKey = contextKey{}

// WithLogger returns a new context with a zap.Logger.
func WithLogger(ctx context.Context, verbose bool) context.Context {
	logger := zap.NewNop()
	if verbose {
		logger = newConsoleLogger()
	}
	return context.WithValue(ctx, ctxKey, logger)
}

func newConsoleLogger() *zap.Logger {
	cnf := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
		Development: true,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "",
			LevelKey:       "",
			NameKey:        "",
			CallerKey:      "",
			MessageKey:     "M",
			StacktraceKey:  "",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := cnf.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %s", err))
	}
	return logger
}

// LoggerFromContext returns a zap.Logger stored in a context.
// If the context does not contain a zap.Logger the function returns a silent zap.Logger.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	l := ctx.Value(ctxKey)
	if l == nil {
		return zap.NewNop()
	}
	return l.(*zap.Logger)
}
