package config_test

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/config"
)

type Faulty struct {
	DatabaseURL string        `env:"DATABASE_URL,required"`
	BrokerURL   string        `env:"BROKER_URL,required"`
	APIKey      string        `env:"API_KEY,required,secret"`
	Grace       time.Duration `env:"GRACE" default:"15s"`
}

func faults(t *testing.T, pairs map[string]string) *config.FaultList {
	t.Helper()
	_, _, err := config.LoadFrom[Faulty](context.Background(), env(pairs))
	if err == nil {
		t.Fatal("LoadFrom accepted a configuration that holds a fault")
	}
	var list *config.FaultList
	if !errors.As(err, &list) {
		t.Fatalf("err is %T, want *config.FaultList", err)
	}
	return list
}

func TestEveryMissingRequiredVariableIsReported(t *testing.T) {
	list := faults(t, nil)
	if len(list.Faults) != 3 {
		t.Fatalf("the loader reported %d faults, want 3: %v", len(list.Faults), list)
	}
	got := map[string]bool{}
	for _, f := range list.Faults {
		got[f.Name] = true
	}
	for _, name := range []string{"DATABASE_URL", "BROKER_URL", "API_KEY"} {
		if !got[name] {
			t.Fatalf("the fault list does not name %s: %v", name, list)
		}
	}
}

func TestEveryFaultStatesTheRepair(t *testing.T) {
	list := faults(t, map[string]string{"GRACE": "soon"})
	if len(list.Faults) == 0 {
		t.Fatal("the loader reported no fault")
	}
	for _, f := range list.Faults {
		if f.Repair == "" {
			t.Fatalf("the fault for %s states no repair", f.Name)
		}
		if strings.Count(f.Repair, ".") > 1 {
			t.Fatalf("the repair for %s is more than one sentence: %q", f.Name, f.Repair)
		}
		if !strings.Contains(f.Error(), f.Repair) {
			t.Fatalf("the message for %s drops the repair: %q", f.Name, f.Error())
		}
	}
}

func TestAMalformedDurationNamesTheVariableAndTheValue(t *testing.T) {
	list := faults(t, map[string]string{
		"DATABASE_URL": "postgres://x",
		"BROKER_URL":   "amqp://x",
		"API_KEY":      "k",
		"GRACE":        "soon",
	})
	if len(list.Faults) != 1 {
		t.Fatalf("the loader reported %d faults, want 1: %v", len(list.Faults), list)
	}
	msg := list.Faults[0].Error()
	if !strings.Contains(msg, "GRACE") {
		t.Fatalf("the message does not name the variable: %q", msg)
	}
	if !strings.Contains(msg, `"soon"`) {
		t.Fatalf("the message does not carry the value it received: %q", msg)
	}
}

func TestAMalformedIntNamesTheVariableAndTheValue(t *testing.T) {
	type Outer struct {
		Count int `env:"COUNT"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"COUNT": "ten"}))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed integer")
	}
	if !strings.Contains(err.Error(), "COUNT") || !strings.Contains(err.Error(), `"ten"`) {
		t.Fatalf("the message is %q", err.Error())
	}
}

func TestAMalformedBoolNamesTheVariableAndTheValue(t *testing.T) {
	type Outer struct {
		Enabled bool `env:"ENABLED"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"ENABLED": "yes-please"}))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed boolean")
	}
	if !strings.Contains(err.Error(), "ENABLED") || !strings.Contains(err.Error(), `"yes-please"`) {
		t.Fatalf("the message is %q", err.Error())
	}
}

func TestAMalformedURLNamesTheVariableAndTheValue(t *testing.T) {
	type Outer struct {
		Endpoint url.URL `env:"ENDPOINT"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"ENDPOINT": "://nope"}))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed URL")
	}
	if !strings.Contains(err.Error(), "ENDPOINT") || !strings.Contains(err.Error(), `"://nope"`) {
		t.Fatalf("the message is %q", err.Error())
	}
}

func TestAFaultNeverPrintsASecretValue(t *testing.T) {
	type Outer struct {
		APIKey config.Secret `env:"API_KEY,secret"`
		Count  int           `env:"COUNT"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{
		"API_KEY": "super-secret-value",
		"COUNT":   "ten",
	}))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed integer")
	}
	if strings.Contains(err.Error(), "super-secret-value") {
		t.Fatalf("the fault message leaks a secret: %q", err.Error())
	}
}

func TestAMalformedSecretValueIsRedactedInItsOwnFault(t *testing.T) {
	type Outer struct {
		Grace time.Duration `env:"GRACE,secret"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"GRACE": "leak-me"}))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed duration")
	}
	if strings.Contains(err.Error(), "leak-me") {
		t.Fatalf("the fault message leaks a secret: %q", err.Error())
	}
	if !strings.Contains(err.Error(), config.Redacted) {
		t.Fatalf("the fault message does not redact the value: %q", err.Error())
	}
}

func TestRequiredAndDefaultTogetherIsAFault(t *testing.T) {
	type Outer struct {
		Name string `env:"NAME,required" default:"n"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(nil))
	if err == nil {
		t.Fatal("LoadFrom accepted a field that is both required and defaulted")
	}
	if !strings.Contains(err.Error(), "Outer.Name") {
		t.Fatalf("the message does not name the struct field: %q", err.Error())
	}
}

func TestAnUnknownTagOptionIsAFault(t *testing.T) {
	type Outer struct {
		Name string `env:"NAME,requried"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(nil))
	if err == nil {
		t.Fatal("LoadFrom accepted an unknown tag option")
	}
	if !strings.Contains(err.Error(), "requried") || !strings.Contains(err.Error(), "Outer.Name") {
		t.Fatalf("the message is %q", err.Error())
	}
}

func TestAnUnsupportedTypeIsAFault(t *testing.T) {
	type Outer struct {
		Ratio float64 `env:"RATIO"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(nil))
	if err == nil {
		t.Fatal("LoadFrom accepted an unsupported type")
	}
	if !strings.Contains(err.Error(), "float64") || !strings.Contains(err.Error(), "Outer.Ratio") {
		t.Fatalf("the message is %q", err.Error())
	}
}

func TestAMalformedDefaultIsAFault(t *testing.T) {
	type Outer struct {
		Grace time.Duration `env:"GRACE" default:"soon"`
	}
	_, _, err := config.LoadFrom[Outer](context.Background(), env(nil))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed default")
	}
	if !strings.Contains(err.Error(), "default") || !strings.Contains(err.Error(), `"soon"`) {
		t.Fatalf("the message is %q", err.Error())
	}
}

func TestExitPrintsEveryFaultAndReturnsOne(t *testing.T) {
	list := faults(t, nil)
	var buf bytes.Buffer
	code := config.Exit(&buf, list)
	if code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	out := buf.String()
	for _, name := range []string{"DATABASE_URL", "BROKER_URL", "API_KEY"} {
		if !strings.Contains(out, name) {
			t.Fatalf("the output does not name %s:\n%s", name, out)
		}
	}
	if strings.Count(out, "→") != 3 {
		t.Fatalf("the output does not carry one repair for each fault:\n%s", out)
	}
}

func TestExitReturnsZeroForNoError(t *testing.T) {
	var buf bytes.Buffer
	if code := config.Exit(&buf, nil); code != 0 {
		t.Fatalf("Exit returned %d, want 0", code)
	}
	if buf.Len() != 0 {
		t.Fatalf("Exit wrote %q, want nothing", buf.String())
	}
}

func TestFaultListUnwrapsToEachFault(t *testing.T) {
	list := faults(t, nil)
	var target *config.Fault
	if !errors.As(error(list), &target) {
		t.Fatal("errors.As does not reach a single fault")
	}
}
