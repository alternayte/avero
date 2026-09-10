package config_test

import (
	"testing"

	"github.com/alternayte/avero/config"
)

// An application configuration embeds BaseConfig, so it carries Base and it
// satisfies the constraint of avero.Serve with no hand-written method.
func TestBaseReturnsTheEmbeddedConfiguration(t *testing.T) {
	type appConfig struct {
		config.BaseConfig
		DatabaseURL string
	}
	cfg := appConfig{BaseConfig: config.BaseConfig{Port: 9090}}
	if got := cfg.Base().Port; got != 9090 {
		t.Fatalf("Base returned the port %d", got)
	}
}
