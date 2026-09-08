package avero_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	avero "github.com/alternayte/avero"
	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/host"
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
	var out strings.Builder
	if code := avero.Exit(&out, err); code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	if !strings.Contains(out.String(), "DATABASE_URL") {
		t.Fatalf("the output does not name the variable:\n%s", out.String())
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
