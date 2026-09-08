package avero_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	avero "github.com/alternayte/avero"
	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/telemetry"
)

// The root package holds aliases and thin wrappers. It holds no logic and no
// state, so a test proves only that the names reach the subsystem.

type appConfig struct {
	avero.BaseConfig
	DatabaseURL string       `env:"DATABASE_URL,required"`
	APIKey      avero.Secret `env:"API_KEY,secret"`
}

type noop struct{ name string }

func (n noop) Name() string                { return n.name }
func (n noop) Start(context.Context) error { return nil }
func (n noop) Stop(context.Context) error  { return nil }
func (n noop) Ready(context.Context) error { return nil }

// Each of these functions takes a subsystem type. A call with the root alias
// compiles only when the alias and the subsystem type are the same type.
func takeBaseConfig(config.BaseConfig) {}
func takeOTelConfig(config.OTelConfig) {}
func takeSecret(config.Secret)         {}
func takeReport(*config.Report)        {}
func takeField(config.Field)           {}
func takeSource(config.Source)         {}
func takeLoader(config.Loader)         {}
func takeApp(*host.App)                {}
func takeCheck(host.Check)             {}
func takeComponent(host.Component)     {}
func takeReady(host.Ready)             {}
func takeMigrator(host.Migrator)       {}
func takeOption(host.Option)           {}

func TestTheAliasesNameTheSubsystemTypes(t *testing.T) {
	takeBaseConfig(avero.BaseConfig{})
	takeOTelConfig(avero.OTelConfig{})
	takeSecret(avero.Secret("x"))
	takeReport((*avero.Report)(nil))
	takeField(avero.Field{})
	takeSource(avero.SourceEnv)
	takeLoader(avero.Loader{})
	takeApp((*avero.App)(nil))
	takeCheck(avero.Check{})
	takeComponent(noop{})
	takeReady(noop{})
	takeMigrator(fakeMigrator{})
	takeOption(avero.WithoutSignals())

	if avero.Redacted != config.Redacted {
		t.Fatalf("Redacted is %q, want %q", avero.Redacted, config.Redacted)
	}
	if avero.SourceDefault != config.SourceDefault || avero.SourceAbsent != config.SourceAbsent {
		t.Fatal("the source constants do not name the config constants")
	}
}

type fakeMigrator struct{}

func (fakeMigrator) Migrate(context.Context) error { return nil }

func TestLoadReadsTheProcessEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("API_KEY", "sk-live-1234")
	t.Setenv("AVERO_SECRET", strings.Repeat("k", 32))
	cfg, err := avero.Load[appConfig](context.Background())
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://x" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want the default 8080", cfg.Port)
	}
	if strings.Contains(strings.ToLower(cfg.APIKey.String()), "sk-live") {
		t.Fatalf("the secret is not redacted: %v", cfg.APIKey)
	}
}

func TestNewAndRunReachTheHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}
	cfg := avero.BaseConfig{ShutdownGrace: 2 * time.Second, LogLevel: "error", LogFormat: "text"}
	app := avero.New(cfg,
		avero.WithoutSignals(),
		avero.WithListener(ln),
		avero.WithComponents(noop{name: "db"}))

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() { errs <- app.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) && !ready {
		res, err := (&http.Client{Timeout: time.Second}).Get("http://" + app.Addr() + "/readyz")
		if err == nil {
			ready = res.StatusCode == http.StatusOK
			_ = res.Body.Close()
		}
		if !ready {
			time.Sleep(2 * time.Millisecond)
		}
	}
	if !ready {
		t.Fatal("the application did not become ready")
	}
	cancel()
	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("Run returned an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestExitPrintsAConfigurationFault(t *testing.T) {
	_, err := avero.Load[appConfig](context.Background())
	if err == nil {
		t.Fatal("Load accepted a missing required variable")
	}
	// Two variables are required and both are absent, so both appear.
	var out strings.Builder
	if code := avero.Exit(&out, err); code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	for _, name := range []string{"DATABASE_URL", "AVERO_SECRET"} {
		if !strings.Contains(out.String(), name) {
			t.Fatalf("the output does not name %s:\n%s", name, out.String())
		}
	}
	if !strings.Contains(out.String(), "→") {
		t.Fatalf("the output states no repair:\n%s", out.String())
	}
}

func TestExitPrintsAHostFault(t *testing.T) {
	err := &host.BootFaults{Faults: []*host.BootFault{{
		Check:  "the database",
		Repair: "Start the database and run the application again",
		Err:    errors.New("dial tcp: refused"),
	}}}
	var out strings.Builder
	if code := avero.Exit(&out, err); code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	if !strings.Contains(out.String(), "the database") {
		t.Fatalf("the output does not name the check:\n%s", out.String())
	}
}

func TestExitReturnsZeroForNoError(t *testing.T) {
	var out strings.Builder
	if code := avero.Exit(&out, nil); code != 0 {
		t.Fatalf("Exit returned %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("Exit wrote %q, want nothing", out.String())
	}
}

func takeRouter(*router.Router)        {}
func takeCtx(*router.Ctx)              {}
func takeResponse(router.Response)     {}
func takeMiddleware(router.Middleware) {}
func takeRoute(router.Route)           {}
func takeToast(router.Toast)           {}

func TestTheRouterAliasesNameTheSubsystemTypes(t *testing.T) {
	takeRouter(avero.NewRouter())
	takeCtx((*avero.Ctx)(nil))
	takeResponse(avero.NoContent())
	takeMiddleware(avero.RequestID())
	takeRoute(avero.Route{})
	takeToast(avero.Toast{Level: avero.ToastInfo, Message: "x"})
	if avero.StatusCSRF != router.StatusCSRF {
		t.Fatal("StatusCSRF does not name the router constant")
	}
}

func TestTheRootResponseConstructorsReachTheRouter(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  avero.Response
		want int
	}{
		{"text", avero.Text(200, "x"), 200},
		{"html", avero.HTML(200, "x"), 200},
		{"json", avero.JSON(201, nil), 201},
		{"redirect", avero.Redirect(303, "/x"), 303},
		{"no content", avero.NoContent(), 204},
		{"empty", avero.Empty(202), 202},
		{"status", avero.Status(422), 422},
	} {
		if got := tc.res.Status(); got != tc.want {
			t.Fatalf("%s reports %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestTheRootRouterServesARoute(t *testing.T) {
	r := avero.NewRouter()
	r.Get("/things", func(*avero.Ctx) (avero.Response, error) {
		return avero.Text(200, "listed"), nil
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things", nil))
	if rec.Code != 200 || rec.Body.String() != "listed" {
		t.Fatalf("gave %d %q", rec.Code, rec.Body.String())
	}
}

func TestSecretCheckPassesAStrongSecret(t *testing.T) {
	c := avero.SecretCheck(avero.Secret(strings.Repeat("k", 32)))
	if c.Name == "" || c.Repair == "" || c.Run == nil {
		t.Fatalf("the check is not complete: %+v", c)
	}
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("the check rejected a 32-byte secret: %v", err)
	}
}

func TestSecretCheckRejectsAShortSecret(t *testing.T) {
	c := avero.SecretCheck(avero.Secret("too-short"))
	err := c.Run(context.Background())
	if err == nil {
		t.Fatal("the check accepted a short secret")
	}
	if strings.Contains(err.Error(), "too-short") {
		t.Fatalf("the fault leaks the secret: %v", err)
	}
	if !strings.Contains(err.Error(), "AVERO_SECRET") {
		t.Fatalf("the fault does not name the variable: %v", err)
	}
}

func TestSecretCheckStopsTheBoot(t *testing.T) {
	// A short secret must stop the process before it serves, and not panic
	// inside the router wiring. See DX-8.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned an error: %v", err)
	}
	defer func() { _ = ln.Close() }()

	app := avero.New(avero.BaseConfig{ShutdownGrace: time.Second, LogLevel: "error"},
		avero.WithoutSignals(),
		avero.WithListener(ln),
		avero.WithChecks(avero.SecretCheck(avero.Secret("too-short"))))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runErr := app.Run(ctx)
	if runErr == nil {
		t.Fatal("the application started with a short secret")
	}
	var out strings.Builder
	if code := avero.Exit(&out, runErr); code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	if !strings.Contains(out.String(), "→") {
		t.Fatalf("the output states no repair:\n%s", out.String())
	}
}

func TestTheTelemetryAliasesNameTheSubsystemTypes(t *testing.T) {
	cfg := avero.BaseConfig{
		LogLevel:  "error",
		LogFormat: "json",
		OTel:      avero.OTelConfig{TracesExporter: "none", MetricsExporter: "none"},
	}
	p, err := avero.NewTelemetry(cfg)
	if err != nil {
		t.Fatalf("NewTelemetry returned an error: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	takeTelemetry(p)
	takeComponent(p)
	if p.TracesEnabled() {
		t.Fatal("the default exporter is not none")
	}
	if got := avero.Attr("api_key", avero.Secret("sk-live-1234")).Value.Emit(); got != avero.Redacted {
		t.Fatalf("Attr gave %q, want %q", got, avero.Redacted)
	}
}

func takeTelemetry(*telemetry.Provider) {}
