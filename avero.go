// Package avero is the name that an application writes.
//
// It holds aliases and thin wrappers for the subsystem packages. It holds no
// logic and no state, so it is not a facade: it hides nothing and it adds no
// behaviour. Read the subsystem package for the contract of a name.
//
//	config   the configuration loader and the report, S1
//	host     the lifecycle, the readiness gate and the boot checks, S2
//	router   the routes, the request context and the middleware, S4
//
// An application can import a subsystem directly. The two forms are the same
// types.
package avero

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/alternayte/drel"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
	"github.com/alternayte/avero/router"
)

// The configuration types. See the config package, S1.
type (
	// BaseConfig holds the variables that every Avero application reads.
	BaseConfig = config.BaseConfig
	// OTelConfig holds the standard OTEL_* variables.
	OTelConfig = config.OTelConfig
	// Secret holds a value that no log line and no error message prints.
	Secret = config.Secret
	// Report lists every configuration field with its value and its source.
	Report = config.Report
	// Field is one row of a report.
	Field = config.Field
	// Source states where a value came from.
	Source = config.Source
	// Loader reads the configuration values.
	Loader = config.Loader
)

// Redacted replaces the value of a secret field wherever Avero prints it.
const Redacted = config.Redacted

// The sources that a report names.
const (
	SourceEnv     = config.SourceEnv
	SourceDefault = config.SourceDefault
	SourceAbsent  = config.SourceAbsent
)

// The lifecycle types. See the host package, S2.
type (
	// App is one Avero application.
	App = host.App
	// Component is a part of the application that has a lifecycle.
	Component = host.Component
	// Ready is the optional readiness contract.
	Ready = host.Ready
	// Check is one boot check.
	Check = host.Check
	// Migrator applies the pending migrations at boot.
	Migrator = host.Migrator
	// Option configures an application.
	Option = host.Option
)

// Load reads the process environment into a new T. See config.Load.
func Load[T any](ctx context.Context) (*T, error) { return config.Load[T](ctx) }

// LoadFrom reads l into a new T and returns a report. See config.LoadFrom.
func LoadFrom[T any](ctx context.Context, l Loader) (*T, *Report, error) {
	return config.LoadFrom[T](ctx, l)
}

// New builds an application. See host.New.
func New(cfg BaseConfig, opts ...Option) *App { return host.New(cfg, opts...) }

// WithComponents registers components. See host.WithComponents.
func WithComponents(cs ...Component) Option { return host.WithComponents(cs...) }

// WithChecks registers boot checks. See host.WithChecks.
func WithChecks(cs ...Check) Option { return host.WithChecks(cs...) }

// WithHandler mounts the application handler. See host.WithHandler.
func WithHandler(h http.Handler) Option { return host.WithHandler(h) }

// WithListener supplies the listener. See host.WithListener.
func WithListener(l net.Listener) Option { return host.WithListener(l) }

// WithMigrator supplies the migrator. See host.WithMigrator.
func WithMigrator(m Migrator) Option { return host.WithMigrator(m) }

// WithLogger supplies the logger. See host.WithLogger.
func WithLogger(l *slog.Logger) Option { return host.WithLogger(l) }

// WithSignals sets the signals that close the application. See host.WithSignals.
func WithSignals(sig ...os.Signal) Option { return host.WithSignals(sig...) }

// WithoutSignals installs no signal handler. See host.WithoutSignals.
func WithoutSignals() Option { return host.WithoutSignals() }

// Exit writes err to w and returns the process exit code. It returns 0 for a
// nil error and 1 for any fault. It prints a configuration fault with the
// config format and every other fault with the host format. The caller owns
// the call to os.Exit.
func Exit(w io.Writer, err error) int {
	if err == nil {
		return 0
	}
	var faults *config.FaultList
	if errors.As(err, &faults) {
		return config.Exit(w, err)
	}
	return host.Exit(w, err)
}

// The routing types. See the router package, S4.
type (
	// Router registers routes and builds the http.Handler.
	Router = router.Router
	// Ctx carries one request.
	Ctx = router.Ctx
	// Handler is one typed handler.
	Handler = router.Handler
	// Response is the result of a handler.
	Response = router.Response
	// Middleware wraps a handler and carries a name.
	Middleware = router.Middleware
	// Route is one registered route.
	Route = router.Route
	// Toast is one message that the next rendered page shows.
	Toast = router.Toast
	// RouteReport lists every route of a router.
	RouteReport = router.Report
)

// The toast levels and the CSRF status.
const (
	ToastInfo    = router.ToastInfo
	ToastSuccess = router.ToastSuccess
	ToastWarning = router.ToastWarning
	ToastError   = router.ToastError
	// StatusCSRF is the status of a CSRF fault.
	StatusCSRF = router.StatusCSRF
)

// NewRouter builds a router. See router.New.
func NewRouter(opts ...router.Option) *Router { return router.New(opts...) }

// The response constructors. See the router package.

// Text returns a plain text response.
func Text(code int, body string) Response { return router.Text(code, body) }

// HTML returns an HTML response.
func HTML(code int, body string) Response { return router.HTML(code, body) }

// JSON returns a response that encodes body as JSON.
func JSON(code int, body any) Response { return router.JSON(code, body) }

// Redirect returns a redirect.
func Redirect(code int, to string) Response { return router.Redirect(code, to) }

// NoContent returns 204 with no body.
func NoContent() Response { return router.NoContent() }

// Empty returns a status with no body.
func Empty(code int) Response { return router.Empty(code) }

// Status returns a response that carries the standard text of the code.
func Status(code int) Response { return router.Status(code) }

// The middleware. See the router package for the scaffolded order.

// RequestID gives each request an identifier.
func RequestID() Middleware { return router.RequestID() }

// Recover turns a panic into 500.
func Recover(l *slog.Logger) Middleware { return router.Recover(l) }

// AccessLog writes one line for each request.
func AccessLog(l *slog.Logger) Middleware { return router.AccessLog(l) }

// CSRF protects an unsafe method with a signed double-submit token.
func CSRF(secret string, opts ...router.CookieOption) Middleware {
	return router.CSRF(secret, opts...)
}

// Flash carries a Toast from one request to the next.
func Flash(secret string, opts ...router.CookieOption) Middleware {
	return router.Flash(secret, opts...)
}

// Transaction opens a drel transaction and decides the commit from the
// response.
func Transaction(e *drel.Engine) Middleware { return router.Transaction(e) }

// Adapt turns a net/http middleware into an Avero middleware.
func Adapt(name string, mw func(http.Handler) http.Handler) Middleware {
	return router.Adapt(name, mw)
}
