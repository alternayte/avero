package config_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
)

const raw = "sk-live-1234567890"

func TestSecretRedactsUnderTheVerbV(t *testing.T) {
	s := config.Secret(raw)
	if got := fmt.Sprintf("%v", s); got != config.Redacted {
		t.Fatalf("%%v gave %q, want %q", got, config.Redacted)
	}
}

func TestSecretRedactsInsideAStructUnderTheVerbV(t *testing.T) {
	type App struct {
		Name   string
		APIKey config.Secret
	}
	out := fmt.Sprintf("%v", App{Name: "orders", APIKey: config.Secret(raw)})
	if strings.Contains(out, raw) {
		t.Fatalf("%%v of the struct leaks the secret: %s", out)
	}
	if !strings.Contains(out, config.Redacted) {
		t.Fatalf("%%v of the struct does not redact: %s", out)
	}
}

func TestSecretRedactsUnderEveryVerb(t *testing.T) {
	s := config.Secret(raw)
	for _, format := range []string{"%s", "%q", "%v", "%+v", "%#v", "%d", "%x"} {
		if strings.Contains(fmt.Sprintf(format, s), raw) {
			t.Fatalf("%s leaks the secret", format)
		}
	}
}

func TestSecretRedactsInJSON(t *testing.T) {
	b, err := json.Marshal(struct {
		APIKey config.Secret `json:"api_key"`
	}{config.Secret(raw)})
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	if strings.Contains(string(b), raw) {
		t.Fatalf("the JSON leaks the secret: %s", b)
	}
}

func TestSecretRedactsInASlogRecord(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewTextHandler(&buf, nil))
	log.Info("boot", "api_key", config.Secret(raw))
	if strings.Contains(buf.String(), raw) {
		t.Fatalf("the log line leaks the secret: %s", buf.String())
	}
}

func TestSecretRevealReturnsTheValue(t *testing.T) {
	if got := config.Secret(raw).Reveal(); got != raw {
		t.Fatalf("Reveal gave %q, want %q", got, raw)
	}
}

func TestAnEmptySecretRedactsToTheEmptyString(t *testing.T) {
	if got := fmt.Sprintf("%v", config.Secret("")); got != "" {
		t.Fatalf("%%v of an empty secret gave %q, want the empty string", got)
	}
}
