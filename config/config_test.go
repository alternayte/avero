package config_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/config"
)

// env builds a loader that reads from a fixed map. It replaces the process
// environment so that a test never mutates global state.
func env(pairs map[string]string) config.Loader {
	return config.Loader{
		Lookup: func(name string) (string, bool) {
			v, ok := pairs[name]
			return v, ok
		},
	}
}

type Nested struct {
	Host string `env:"HOST" default:"localhost"`
	Port int    `env:"PORT" default:"5432"`
}

type AllTypes struct {
	Name     string        `env:"NAME"`
	Count    int           `env:"COUNT" default:"3"`
	Big      int64         `env:"BIG" default:"9007199254740993"`
	Enabled  bool          `env:"ENABLED" default:"true"`
	Grace    time.Duration `env:"GRACE" default:"15s"`
	Endpoint url.URL       `env:"ENDPOINT" default:"https://example.com/v1"`
	Hosts    []string      `env:"HOSTS" default:"a,b"`
	DB       Nested        `env:"DB"`
}

func loadAll(t *testing.T, pairs map[string]string) (*AllTypes, *config.Report) {
	t.Helper()
	cfg, rep, err := config.LoadFrom[AllTypes](context.Background(), env(pairs))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	return cfg, rep
}

func TestParseString(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"NAME": "avero"})
	if cfg.Name != "avero" {
		t.Fatalf("Name = %q, want %q", cfg.Name, "avero")
	}
}

func TestParseInt(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"COUNT": "42"})
	if cfg.Count != 42 {
		t.Fatalf("Count = %d, want 42", cfg.Count)
	}
}

func TestParseIntDefault(t *testing.T) {
	cfg, _ := loadAll(t, nil)
	if cfg.Count != 3 {
		t.Fatalf("Count = %d, want the default 3", cfg.Count)
	}
}

func TestParseInt64(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"BIG": "-9007199254740993"})
	if cfg.Big != -9007199254740993 {
		t.Fatalf("Big = %d, want -9007199254740993", cfg.Big)
	}
}

func TestParseInt64Default(t *testing.T) {
	cfg, _ := loadAll(t, nil)
	if cfg.Big != 9007199254740993 {
		t.Fatalf("Big = %d, want the default", cfg.Big)
	}
}

func TestParseBool(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"true", true}, {"TRUE", true}, {"1", true},
		{"false", false}, {"FALSE", false}, {"0", false},
	} {
		cfg, _ := loadAll(t, map[string]string{"ENABLED": tc.in})
		if cfg.Enabled != tc.want {
			t.Fatalf("ENABLED=%q gave %v, want %v", tc.in, cfg.Enabled, tc.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"GRACE": "1m30s"})
	if cfg.Grace != 90*time.Second {
		t.Fatalf("Grace = %v, want 1m30s", cfg.Grace)
	}
}

func TestParseDurationDefault(t *testing.T) {
	cfg, _ := loadAll(t, nil)
	if cfg.Grace != 15*time.Second {
		t.Fatalf("Grace = %v, want the default 15s", cfg.Grace)
	}
}

func TestParseURL(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"ENDPOINT": "https://otel.example.com:4318/v1/traces"})
	if cfg.Endpoint.Host != "otel.example.com:4318" {
		t.Fatalf("Endpoint.Host = %q", cfg.Endpoint.Host)
	}
	if cfg.Endpoint.Path != "/v1/traces" {
		t.Fatalf("Endpoint.Path = %q", cfg.Endpoint.Path)
	}
	if cfg.Endpoint.Scheme != "https" {
		t.Fatalf("Endpoint.Scheme = %q", cfg.Endpoint.Scheme)
	}
}

func TestParseURLDefault(t *testing.T) {
	cfg, _ := loadAll(t, nil)
	if cfg.Endpoint.String() != "https://example.com/v1" {
		t.Fatalf("Endpoint = %q", cfg.Endpoint.String())
	}
}

func TestParseSlice(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"HOSTS": "one, two ,three"})
	want := []string{"one", "two", "three"}
	if len(cfg.Hosts) != len(want) {
		t.Fatalf("Hosts = %#v, want %#v", cfg.Hosts, want)
	}
	for i := range want {
		if cfg.Hosts[i] != want[i] {
			t.Fatalf("Hosts = %#v, want %#v", cfg.Hosts, want)
		}
	}
}

func TestParseSliceEmptyGivesNoElement(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"HOSTS": ""})
	if len(cfg.Hosts) != 0 {
		t.Fatalf("Hosts = %#v, want no element", cfg.Hosts)
	}
}

func TestNestedStructTakesAPrefix(t *testing.T) {
	cfg, _ := loadAll(t, map[string]string{"DB_HOST": "db.internal", "DB_PORT": "6543"})
	if cfg.DB.Host != "db.internal" {
		t.Fatalf("DB.Host = %q", cfg.DB.Host)
	}
	if cfg.DB.Port != 6543 {
		t.Fatalf("DB.Port = %d", cfg.DB.Port)
	}
}

func TestNestedStructUsesItsDefaults(t *testing.T) {
	cfg, _ := loadAll(t, nil)
	if cfg.DB.Host != "localhost" || cfg.DB.Port != 5432 {
		t.Fatalf("DB = %+v, want the defaults", cfg.DB)
	}
}

func TestNestedPrefixDerivesFromTheFieldName(t *testing.T) {
	type Inner struct {
		Key string `env:"KEY"`
	}
	type Outer struct {
		ObjectStore Inner
	}
	cfg, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"OBJECT_STORE_KEY": "k"}))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.ObjectStore.Key != "k" {
		t.Fatalf("ObjectStore.Key = %q, want %q", cfg.ObjectStore.Key, "k")
	}
}

func TestEnvNameDerivesFromTheFieldName(t *testing.T) {
	type Outer struct {
		DatabaseURL string
		MaxRetries  int `default:"2"`
	}
	cfg, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://x" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.MaxRetries != 2 {
		t.Fatalf("MaxRetries = %d, want 2", cfg.MaxRetries)
	}
}

func TestEmbeddedStructTakesNoPrefix(t *testing.T) {
	type App struct {
		config.BaseConfig
		DatabaseURL string `env:"DATABASE_URL,required"`
	}
	cfg, _, err := config.LoadFrom[App](context.Background(), env(map[string]string{
		"DATABASE_URL": "postgres://x",
		"PORT":         "9090",
	}))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("Port = %d, want 9090", cfg.Port)
	}
}

func TestSkippedFieldIsNotRead(t *testing.T) {
	type Outer struct {
		Internal string `env:"-"`
		Name     string `env:"NAME" default:"n"`
	}
	cfg, rep, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"INTERNAL": "leak"}))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.Internal != "" {
		t.Fatalf("Internal = %q, want the empty string", cfg.Internal)
	}
	if _, ok := rep.Field("Internal"); ok {
		t.Fatal("the report holds a row for a skipped field")
	}
}

func TestUnexportedFieldIsNotRead(t *testing.T) {
	type Outer struct {
		Name string `env:"NAME" default:"n"`
		//nolint:unused // the loader must step over an unexported field.
		hidden string
	}
	if _, _, err := config.LoadFrom[Outer](context.Background(), env(nil)); err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
}

func TestBaseConfigDefaults(t *testing.T) {
	cfg, _, err := config.LoadFrom[config.BaseConfig](context.Background(), env(nil))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.ShutdownGrace != 15*time.Second {
		t.Fatalf("ShutdownGrace = %v, want 15s", cfg.ShutdownGrace)
	}
	if cfg.MigrateOnBoot {
		t.Fatal("MigrateOnBoot = true, want false")
	}
	if cfg.Env != "development" {
		t.Fatalf("Env = %q, want %q", cfg.Env, "development")
	}
}

func TestBaseConfigReadsTheOTelVariables(t *testing.T) {
	cfg, _, err := config.LoadFrom[config.BaseConfig](context.Background(), env(map[string]string{
		"OTEL_SERVICE_NAME":           "orders",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
		"OTEL_TRACES_EXPORTER":        "otlp",
		"OTEL_METRICS_EXPORTER":       "otlp",
	}))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.OTel.ServiceName != "orders" {
		t.Fatalf("OTel.ServiceName = %q", cfg.OTel.ServiceName)
	}
	if cfg.OTel.ExporterOTLPEndpoint.Host != "collector:4318" {
		t.Fatalf("OTel.ExporterOTLPEndpoint = %q", cfg.OTel.ExporterOTLPEndpoint.String())
	}
	if cfg.OTel.ExporterOTLPProtocol != "http/protobuf" {
		t.Fatalf("OTel.ExporterOTLPProtocol = %q", cfg.OTel.ExporterOTLPProtocol)
	}
	if cfg.OTel.TracesExporter != "otlp" || cfg.OTel.MetricsExporter != "otlp" {
		t.Fatalf("OTel exporters = %q and %q", cfg.OTel.TracesExporter, cfg.OTel.MetricsExporter)
	}
}

func TestBaseConfigDefaultExporterIsNone(t *testing.T) {
	cfg, _, err := config.LoadFrom[config.BaseConfig](context.Background(), env(nil))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	if cfg.OTel.TracesExporter != "none" || cfg.OTel.MetricsExporter != "none" {
		t.Fatalf("the default exporter is %q and %q, want none", cfg.OTel.TracesExporter, cfg.OTel.MetricsExporter)
	}
}

func TestLoadStopsOnACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := config.LoadFrom[AllTypes](ctx, env(nil)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestLoadReadsTheProcessEnvironment(t *testing.T) {
	type Outer struct {
		Name string `env:"AVERO_TEST_NAME"`
	}
	t.Setenv("AVERO_TEST_NAME", "from-the-process")
	cfg, err := config.Load[Outer](context.Background())
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if cfg.Name != "from-the-process" {
		t.Fatalf("Name = %q", cfg.Name)
	}
}

func TestATargetThatIsNotAStructIsAFault(t *testing.T) {
	_, _, err := config.LoadFrom[int](context.Background(), env(nil))
	if err == nil {
		t.Fatal("LoadFrom accepted a target that is not a struct")
	}
	if !strings.Contains(err.Error(), "int") {
		t.Fatalf("the message does not name the type: %v", err)
	}
}
