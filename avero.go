// Package avero is the name that an application writes.
//
// It holds aliases and thin wrappers for the subsystem packages. It holds no
// logic and no state, so it is not a facade: it hides nothing and it adds no
// behaviour. Read the subsystem package for the contract of a name.
//
//	config   the configuration loader and the report, S1
//	host     the lifecycle, the readiness gate and the boot checks, S2
//	router   the routes, the request context and the middleware, S4
//	module   the module contract and the module inspection, S6
//	view     the helpers of a server rendered page, S10
//	assets   the asset pipeline and the manifest, S11
//
// The hypermedia adapters, ds and htmx, are optional imports. This package
// holds no alias for them, so an application that needs neither carries
// neither. See the SDD, S12.
//
//	telemetry the traces, the metrics and the logger, S3
//
// An application can import a subsystem directly. The two forms are the same
// types.
package avero

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/alternayte/drel"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
	"github.com/alternayte/avero/module"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/telemetry"
	"github.com/alternayte/avero/view"
	"go.opentelemetry.io/otel/attribute"
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
	// Fields holds one message for each field that failed validation.
	Fields = router.Fields
	// Binder fills itself from a request. `avero generate` writes it.
	Binder = router.Binder
	// Validator checks itself. `avero generate` writes it.
	Validator = router.Validator
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

// NewCtx builds a Ctx for one request. A test calls a handler with it and
// needs no router. See router.NewCtx.
func NewCtx(w http.ResponseWriter, r *http.Request) *Ctx { return router.NewCtx(w, r) }

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

// Problem is one error of an API, in the shape that RFC 9457 states. See
// router.Problem.
type Problem = router.Problem

// ProblemContentType is the media type of a problem document.
const ProblemContentType = router.ProblemContentType

// The problems that a service answers most. Compare with errors.Is.
var (
	// ErrNotFound states that the path names no thing. 404.
	ErrNotFound = router.ErrNotFound
	// ErrUnauthorized states that the request carries no identity. 401.
	ErrUnauthorized = router.ErrUnauthorized
	// ErrForbidden states that the identity may not do this. 403.
	ErrForbidden = router.ErrForbidden
	// ErrConflict states that the state of the thing refuses the change. 409.
	ErrConflict = router.ErrConflict
	// ErrBadRequest states that the request does not read. 400.
	ErrBadRequest = router.ErrBadRequest
	// ErrInternal states a fault of the service. 500.
	ErrInternal = router.ErrInternal
)

// NewProblem returns a problem with the standard title of the status. See
// router.NewProblem.
func NewProblem(code int, detail string) *Problem { return router.NewProblem(code, detail) }

// NotFound returns the problem of a thing that the table does not hold.
//
//	return nil, avero.NotFound("post", in.ID)
func NotFound(thing, id string) *Problem { return router.NotFound(thing, id) }

// Conflict returns the problem of a state that refuses the change.
func Conflict(detail string) *Problem { return router.Conflict(detail) }

// Unauthorized returns the problem of a request with no identity.
func Unauthorized(detail string) *Problem { return router.Unauthorized(detail) }

// Forbidden returns the problem of an identity that may not do this.
func Forbidden(detail string) *Problem { return router.Forbidden(detail) }

// BadRequest returns the problem of a request that does not read.
func BadRequest(detail string) *Problem { return router.BadRequest(detail) }

// ProblemOf returns the problem that an error carries. See router.ProblemOf.
func ProblemOf(err error) *Problem { return router.ProblemOf(err) }

// Result is the answer of a typed handler. The type argument names the body,
// so the compiler holds the shape of the answer and the description of the API
// reads it. See router.Result.
type Result[T any] = router.Result[T]

// NoBody is the body of an answer that carries none.
type NoBody = router.NoBody

// OK returns 200 with this body.
func OK[T any](body T) Result[T] { return router.OK(body) }

// Created returns 201 with this body.
func Created[T any](body T) Result[T] { return router.Created(body) }

// Done returns 204 and no body.
func Done() Result[NoBody] { return router.Done() }

// OpOption states one more fact of an operation, such as a tag or another
// answer.
type OpOption = router.OpOption

// Summary names the operation for a person.
func Summary(text string) OpOption { return router.Summary(text) }

// Deprecated marks an operation that a caller must leave.
func Deprecated() OpOption { return router.Deprecated() }

// Tags group the operations of a description.
func Tags(names ...string) OpOption { return router.Tags(names...) }

// Answers states one more status that the operation writes.
//
//	avero.Post(r, "/posts", m.Create, avero.Answers[avero.Problem](409, "the title is taken"))
func Answers[T any](code int, description string) OpOption {
	return router.Answers[T](code, description)
}

// AnswersNothing states one more status that the operation writes with no
// body.
func AnswersNothing(code int, description string) OpOption {
	return router.AnswersNothing(code, description)
}

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

// CSRF protects an unsafe method with a signed double-submit token. Pass
// cfg.Secret, which BaseConfig reads from AVERO_SECRET.
func CSRF(secret Secret, opts ...router.CookieOption) Middleware {
	return router.CSRF(secret, opts...)
}

// Flash carries a Toast from one request to the next. Pass cfg.Secret, which
// BaseConfig reads from AVERO_SECRET.
func Flash(secret Secret, opts ...router.CookieOption) Middleware {
	return router.Flash(secret, opts...)
}

// Transaction opens a drel transaction and decides the commit from the
// response.
func Transaction(e *drel.Engine) Middleware { return router.Transaction(e) }

// Adapt turns a net/http middleware into an Avero middleware.
func Adapt(name string, mw func(http.Handler) http.Handler) Middleware {
	return router.Adapt(name, mw)
}

// SecretCheck returns a boot check that proves AVERO_SECRET is long enough to
// sign a cookie. Register it with WithChecks, so that a weak secret stops the
// process with a repair sentence instead of a panic inside the router wiring.
// See DX-8.
//
//	app := avero.New(cfg.BaseConfig,
//	    avero.WithChecks(avero.SecretCheck(cfg.Secret)),
//	    avero.WithHandler(handler))
func SecretCheck(secret Secret) Check {
	return Check{
		Name: "the length of AVERO_SECRET",
		Repair: fmt.Sprintf(
			"Set AVERO_SECRET to at least %d bytes, for example the output of `openssl rand -hex 32`",
			router.MinSecretLength),
		Run: func(context.Context) error {
			if n := len(secret.Reveal()); n < router.MinSecretLength {
				return fmt.Errorf("AVERO_SECRET holds %d bytes and it needs %d",
					n, router.MinSecretLength)
			}
			return nil
		},
	}
}

// The telemetry types. See the telemetry package, S3.
type (
	// Telemetry holds the tracer, the meter and the logger of one
	// application. It is a component, so Stop flushes the exporters.
	Telemetry = telemetry.Provider
	// TelemetryOption configures the telemetry provider.
	TelemetryOption = telemetry.Option
)

// NewTelemetry builds the telemetry provider from the OTEL_* variables. See
// telemetry.New.
func NewTelemetry(cfg BaseConfig, opts ...TelemetryOption) (*Telemetry, error) {
	return telemetry.New(cfg, opts...)
}

// Attr builds a span attribute. A Secret reads as ******** and never reaches
// an exporter. See telemetry.Attr.
func Attr(key string, value any) attribute.KeyValue { return telemetry.Attr(key, value) }

// In adapts a typed handler to the router. The constraint requires the Bind
// and Validate methods that `avero generate` writes, so a missing or stale
// generated file is a compile fault at the route. See router.In.
//
//	r.Post("/things", avero.In(m.Create))
func In[T any, P interface {
	*T
	Binder
	Validator
}](fn func(c *Ctx, in T) (Response, error),
) Handler {
	return router.In[T, P](fn)
}

// FieldsFrom returns the validation result of a failed request. S10 renders
// the form again from it.
func FieldsFrom(ctx context.Context) *Fields { return router.FieldsFrom(ctx) }

// OldFrom returns the input that failed validation.
func OldFrom(ctx context.Context) any { return router.OldFrom(ctx) }

// The module types. See the module package, S6.
type (
	// Module is one feature of an application.
	Module = module.Module
	// HTTPModule contributes routes.
	HTTPModule = module.HTTPModule
	// InboxModule contributes message handlers.
	InboxModule = module.InboxModule
	// InboxHandler is one message handler.
	InboxHandler = module.InboxHandler
	// ScheduleModule contributes scheduled jobs.
	ScheduleModule = module.ScheduleModule
	// ProjectorModule contributes projections.
	ProjectorModule = module.ProjectorModule
	// Projection is one read model projection.
	Projection = module.Projection
	// MigrationModule contributes migration files.
	MigrationModule = module.MigrationModule
	// DescribeModule states the description that an agent reads.
	DescribeModule = module.DescribeModule
	// Description states what one module contributes. See AN-3.
	Description = module.Description
	// RouteDesc is one route of a module.
	RouteDesc = module.RouteDesc
	// ModelDesc is one persistent model of a module.
	ModelDesc = module.ModelDesc
	// EventDesc is one event of a module.
	EventDesc = module.EventDesc
	// FieldDesc is one field of a model or of an event.
	FieldDesc = module.FieldDesc
	// Scheduler collects the jobs of the modules.
	Scheduler = module.Scheduler
	// Job is one scheduled unit of work.
	Job = module.Job
	// ModuleSet holds the modules and the result of the inspection.
	ModuleSet = module.Set
	// ModuleReport lists the contribution of every module.
	ModuleReport = module.Report
	// Contribution is one row of the contribution table.
	Contribution = module.Contribution
)

// Modules inspects each module one time and returns the set. Attach adds the
// routes to a router. See module.Modules.
//
//	set := avero.Modules(billing.New(db), catalog.New(db))
//	if err := set.Attach(r); err != nil {
//	    os.Exit(avero.Exit(os.Stderr, err))
//	}
func Modules(ms ...Module) *ModuleSet { return module.Modules(ms...) }

// The view types. See the view package, S10.
type (
	// ViewComponent renders itself. A templ component satisfies it. The
	// root package already holds Component for the lifecycle contract of
	// the host, so the view name carries the View prefix here.
	ViewComponent = view.Component
	// ViewFunc adapts a function to the Component interface.
	ViewFunc = view.Func
)

// View renders a component with 200 and the HTML content type.
//
//	return avero.View(pages.Index(posts)), nil
func View(c ViewComponent) Response { return view.View(c) }

// ViewStatus renders a component with this status. See view.ViewStatus.
func ViewStatus(code int, c ViewComponent) Response { return view.Status(code, c) }

// WithForm renders the form again with 422 when validation fails. Pass it to
// NewRouter. See view.WithForm.
func WithForm(page func(c *Ctx, f *Fields) ViewComponent) router.Option {
	return view.WithForm(page)
}

// Old returns the value that the person submitted for this field. See
// view.Old.
func Old(ctx context.Context, field string) string { return view.Old(ctx, field) }

// FieldError returns the validation message of this field.
//
// The SDD names this helper Error. The root package already holds the Error
// name in the toast API of Ctx, and a package level Error reads as a fault
// type in Go. The view package holds the name that the SDD states, and a
// component calls view.Error.
func FieldError(ctx context.Context, field string) string { return view.Error(ctx, field) }

// HasError reports whether this field failed validation. See view.HasError.
func HasError(ctx context.Context, field string) bool { return view.HasError(ctx, field) }

// Toasts returns the messages of this response. See view.Toasts.
func Toasts(ctx context.Context) []Toast { return view.Toasts(ctx) }

// CSRFField returns the hidden field that an unsafe form must carry.
//
// The SDD names it CSRF. The root package already holds CSRF for the
// middleware, so the field carries the longer name here. A component calls
// view.CSRF.
func CSRFField() ViewComponent { return view.CSRF() }

// The asset types. See the assets package, S11.
type (
	// Manifest maps the name of an asset to the built file.
	Manifest = assets.Manifest
	// AssetEntry is one built asset.
	AssetEntry = assets.Entry
)

// LoadManifest reads the asset manifest from an embedded file system. See
// assets.LoadManifest.
//
//	//go:embed all:assets/dist
//	var dist embed.FS
//	m, err := avero.LoadManifest(dist, "assets/dist/manifest.json")
func LoadManifest(fsys fs.FS, name string) (*Manifest, error) {
	return assets.LoadManifest(fsys, name)
}

// AssetHandler serves the built assets of a manifest. See assets.Handler.
//
//	r.Mount("/assets/", avero.AssetHandler(dist, m))
func AssetHandler(fsys fs.FS, m *Manifest) http.Handler { return assets.Handler(fsys, m) }
