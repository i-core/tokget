/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package log

import (
	"context"
	"fmt"
)

type contextKey struct{}

var ctxKey = contextKey{}

// Debugger is a logger that prints debug information.
type Debugger struct {
	enable bool
}

// Debugln formats using the default formats for its operands and print to standard output if enabled.
func (d *Debugger) Debugln(args ...interface{}) {
	if d.enable {
		fmt.Println(args...)
	}
}

// Debugf formats according to a format specifier and writes to standard output if enabled.
func (d *Debugger) Debugf(format string, args ...interface{}) {
	if d.enable {
		fmt.Printf(format, args...)
	}
}

var (
	silentDebugger = &Debugger{enable: false}
	// VerboseDebugger is a Debugger that allowed prints messages to standard output.
	VerboseDebugger = &Debugger{enable: true}
)

// WithDebugger returns a new context with a Debugger.
func WithDebugger(ctx context.Context, l *Debugger) context.Context {
	return context.WithValue(ctx, ctxKey, l)
}

// DebuggerFromContext returns a Debugger stored in a context.
// If the context does not contain a Debugger the function returns a silent Debugger.
func DebuggerFromContext(ctx context.Context) *Debugger {
	l := ctx.Value(ctxKey)
	if l == nil {
		return silentDebugger
	}
	return l.(*Debugger)
}
