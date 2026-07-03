package mermaidascii

// noopLogger is a drop-in, do-nothing replacement for the logrus logger the
// upstream render path used only for Debug/Warn tracing. Vendoring keeps the
// graph algorithm byte-for-byte; the logging is inert (this is a rendering
// library embedded in a TUI, not a service).
type noopLogger struct{}

func (noopLogger) Debug(args ...any)                 {}
func (noopLogger) Debugf(format string, args ...any) {}
func (noopLogger) Info(args ...any)                  {}
func (noopLogger) Infof(format string, args ...any)  {}
func (noopLogger) Warn(args ...any)                  {}
func (noopLogger) Warnf(format string, args ...any)  {}
func (noopLogger) Error(args ...any)                 {}
func (noopLogger) Errorf(format string, args ...any) {}

// log is the package-level logger the vendored files call (was the logrus
// package alias upstream).
var log = noopLogger{}
