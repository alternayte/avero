package host

import (
	"log/slog"
	"net"
	"net/http"
	"os"
)

// Option configures an application. New applies each option in order.
type Option func(*App)

// WithComponents registers components. Avero starts them in this order and
// stops them in reverse order. Call it more than one time to append.
func WithComponents(cs ...Component) Option {
	return func(a *App) { a.components = append(a.components, cs...) }
}

// WithChecks registers boot checks. Avero runs every check before it starts
// the first component. See DX-8.
func WithChecks(cs ...Check) Option {
	return func(a *App) { a.checks = append(a.checks, cs...) }
}

// WithHandler mounts the application handler. Avero serves /readyz itself, so
// the handler never sees that path.
func WithHandler(h http.Handler) Option {
	return func(a *App) { a.handler = h }
}

// WithListener supplies the listener. Avero binds the port from the
// configuration when a test does not supply one.
func WithListener(l net.Listener) Option {
	return func(a *App) { a.ln = l }
}

// WithMigrator supplies the migrator. Avero calls it at boot when
// MIGRATE_ON_BOOT is true.
func WithMigrator(m Migrator) Option {
	return func(a *App) { a.migrator = m }
}

// WithLogger supplies the logger. The default logger reads LOG_LEVEL and
// LOG_FORMAT and writes to standard error.
func WithLogger(l *slog.Logger) Option {
	return func(a *App) { a.log = l }
}

// WithSignals sets the signals that close the application. The default is
// interrupt and terminate.
func WithSignals(sig ...os.Signal) Option {
	return func(a *App) { a.signals = sig }
}

// WithoutSignals installs no signal handler. A test uses it, so that a signal
// to the test binary does not reach the application. See the SDD, S2.
func WithoutSignals() Option {
	return func(a *App) { a.signals = nil }
}
