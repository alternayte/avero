package config_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
)

type Reported struct {
	DatabaseURL string        `env:"DATABASE_URL,required"`
	Port        int           `env:"PORT" default:"8080"`
	APIKey      config.Secret `env:"API_KEY,secret"`
	Region      string        `env:"REGION"`
	DB          Nested        `env:"DB"`
}

func report(t *testing.T) *config.Report {
	t.Helper()
	_, rep, err := config.LoadFrom[Reported](context.Background(), env(map[string]string{
		"DATABASE_URL": "postgres://user:pass@host/db",
		"API_KEY":      "sk-live-1234567890",
	}))
	if err != nil {
		t.Fatalf("LoadFrom returned an error: %v", err)
	}
	return rep
}

func TestReportNamesTheSourceOfEachValue(t *testing.T) {
	rep := report(t)
	for _, tc := range []struct {
		path string
		want config.Source
	}{
		{"DatabaseURL", config.SourceEnv},
		{"Port", config.SourceDefault},
		{"APIKey", config.SourceEnv},
		{"Region", config.SourceAbsent},
		{"DB.Host", config.SourceDefault},
	} {
		f, ok := rep.Field(tc.path)
		if !ok {
			t.Fatalf("the report holds no row for %s", tc.path)
		}
		if f.Source != tc.want {
			t.Fatalf("%s has source %q, want %q", tc.path, f.Source, tc.want)
		}
	}
}

func TestReportHoldsOneRowForEachField(t *testing.T) {
	rep := report(t)
	want := []string{"DatabaseURL", "Port", "APIKey", "Region", "DB.Host", "DB.Port"}
	if len(rep.Fields) != len(want) {
		t.Fatalf("the report holds %d rows, want %d", len(rep.Fields), len(want))
	}
	for i, path := range want {
		if rep.Fields[i].Path != path {
			t.Fatalf("row %d is %q, want %q", i, rep.Fields[i].Path, path)
		}
	}
}

func TestReportNamesTheEnvironmentVariableOfEachRow(t *testing.T) {
	rep := report(t)
	f, ok := rep.Field("DB.Port")
	if !ok {
		t.Fatal("the report holds no row for DB.Port")
	}
	if f.Name != "DB_PORT" {
		t.Fatalf("DB.Port reads %q, want DB_PORT", f.Name)
	}
}

func TestReportRedactsASecret(t *testing.T) {
	rep := report(t)
	f, ok := rep.Field("APIKey")
	if !ok {
		t.Fatal("the report holds no row for APIKey")
	}
	if f.Value != config.Redacted {
		t.Fatalf("APIKey reads %q, want %q", f.Value, config.Redacted)
	}
	if !f.Secret {
		t.Fatal("APIKey is not marked as a secret")
	}
	if strings.Contains(rep.String(), "sk-live-1234567890") {
		t.Fatalf("the report leaks a secret:\n%s", rep.String())
	}
	if !strings.Contains(rep.String(), config.Redacted) {
		t.Fatalf("the report does not print the redaction:\n%s", rep.String())
	}
}

func TestReportJSONRedactsASecret(t *testing.T) {
	rep := report(t)
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	if strings.Contains(string(b), "sk-live-1234567890") {
		t.Fatalf("the JSON report leaks a secret: %s", b)
	}
}

func TestReportPrintsOneRowForEachField(t *testing.T) {
	rep := report(t)
	lines := strings.Split(strings.TrimSpace(rep.String()), "\n")
	// One header row and one row for each field.
	if len(lines) != len(rep.Fields)+1 {
		t.Fatalf("the report printed %d lines, want %d:\n%s", len(lines), len(rep.Fields)+1, rep.String())
	}
}

func TestReportJSONIsStableAcrossRuns(t *testing.T) {
	first, err := json.Marshal(report(t))
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	for range 5 {
		next, err := json.Marshal(report(t))
		if err != nil {
			t.Fatalf("Marshal returned an error: %v", err)
		}
		if string(first) != string(next) {
			t.Fatalf("the JSON report changed between runs:\n%s\n%s", first, next)
		}
	}
}

func TestReportCarriesTheSchemaIdentifier(t *testing.T) {
	b, err := json.Marshal(report(t))
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var doc struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	if doc.Schema != config.ReportSchemaID {
		t.Fatalf("schema = %q, want %q", doc.Schema, config.ReportSchemaID)
	}
}

func TestReportNamesTheTypeOfEachRow(t *testing.T) {
	rep := report(t)
	f, ok := rep.Field("Port")
	if !ok {
		t.Fatal("the report holds no row for Port")
	}
	if f.Type != "int" {
		t.Fatalf("Port has type %q, want int", f.Type)
	}
}

func TestReportRendersAnAbsentValueAsAnEmptyString(t *testing.T) {
	rep := report(t)
	f, _ := rep.Field("Region")
	if f.Value != "" {
		t.Fatalf("Region reads %q, want the empty string", f.Value)
	}
}

func TestReportFormatsWithTheVerbV(t *testing.T) {
	rep := report(t)
	out := fmt.Sprintf("%v", rep)
	if strings.Contains(out, "sk-live-1234567890") {
		t.Fatalf("%%v of the report leaks a secret:\n%s", out)
	}
}

func TestTheFaultListCarriesTheReport(t *testing.T) {
	_, rep, err := config.LoadFrom[Reported](context.Background(), env(nil))
	if err == nil {
		t.Fatal("LoadFrom accepted a missing required variable")
	}
	if rep == nil {
		t.Fatal("LoadFrom returned no report beside the faults")
	}
	var list *config.FaultList
	if !errors.As(err, &list) {
		t.Fatalf("err is %T, want *config.FaultList", err)
	}
	if list.Report == nil {
		t.Fatal("the fault list carries no report")
	}
	f, ok := list.Report.Field("Port")
	if !ok {
		t.Fatal("the report holds no row for Port")
	}
	if f.Source != config.SourceDefault {
		t.Fatalf("Port has source %q, want default", f.Source)
	}
}

func TestAReportRowSurvivesAParseFault(t *testing.T) {
	type Outer struct {
		Count int    `env:"COUNT"`
		Name  string `env:"NAME" default:"n"`
	}
	_, rep, err := config.LoadFrom[Outer](context.Background(), env(map[string]string{"COUNT": "ten"}))
	if err == nil {
		t.Fatal("LoadFrom accepted a malformed integer")
	}
	if _, ok := rep.Field("Count"); !ok {
		t.Fatal("the report drops the row of a field that failed to parse")
	}
	if _, ok := rep.Field("Name"); !ok {
		t.Fatal("the report drops a row that follows a fault")
	}
}
