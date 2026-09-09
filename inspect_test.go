package avero_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	avero "github.com/alternayte/avero"
	"github.com/alternayte/drel"
)

// sqlite returns an engine on a file of a temporary directory.
func sqlite(t *testing.T) (*drel.Engine, string) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "check.db")
	e, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatalf("the database does not open: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e, dsn
}

func TestTheDatabaseCheckHoldsAndFails(t *testing.T) {
	_, dsn := sqlite(t)
	check := avero.DatabaseCheck(dsn)
	if err := check.Run(context.Background()); err != nil {
		t.Fatalf("the check returned %v, want nil", err)
	}
	if check.Repair == "" {
		t.Fatal("the check states no repair")
	}
	absent := avero.DatabaseCheck("postgres://127.0.0.1:1/none?connect_timeout=1")
	if err := absent.Run(context.Background()); err == nil {
		t.Fatal("the check accepted a database that does not answer")
	}
}

func TestTheMigrationCheckReportsAPendingMigration(t *testing.T) {
	engine, dsn := sqlite(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "20250101000000_create_posts.up.sql"),
		[]byte("CREATE TABLE posts (id TEXT PRIMARY KEY);"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}

	check := avero.MigrationCheck(dsn, dir)
	err := check.Run(context.Background())
	if err == nil {
		t.Fatal("the check accepted a database with a pending migration")
	}
	if !strings.Contains(err.Error(), "20250101000000_create_posts") {
		t.Fatalf("the message is %q", err)
	}
	if !strings.Contains(check.Repair, "avero migrate up") {
		t.Fatalf("the repair is %q", check.Repair)
	}

	if _, err := engine.ApplyMigrations(context.Background(), dir); err != nil {
		t.Fatalf("the migrations do not apply: %v", err)
	}
	if err := check.Run(context.Background()); err != nil {
		t.Fatalf("the check returned %v after the migration, want nil", err)
	}
}

func TestTheBrokerCheckReportsAnUnreachableBroker(t *testing.T) {
	// A closed port stands for a broker that does not run.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned %v", err)
	}
	address := "amqp://guest:guest@" + l.Addr().String() + "/"
	check := avero.BrokerCheck(address)
	if err := check.Run(context.Background()); err != nil {
		t.Fatalf("the check returned %v for a broker that answers, want nil", err)
	}
	_ = l.Close()

	err = check.Run(context.Background())
	if err == nil {
		t.Fatal("the check accepted a broker that does not answer")
	}
	if !strings.Contains(err.Error(), "does not answer") {
		t.Fatalf("the message is %q", err)
	}
	if !strings.Contains(check.Repair, "AMQP_URL") {
		t.Fatalf("the repair is %q", check.Repair)
	}
}

func TestInspectingReadsThePrefix(t *testing.T) {
	if !avero.Inspecting([]string{avero.InspectPrefix + "routes"}) {
		t.Fatal("Inspecting = false for an inspection command")
	}
	if avero.Inspecting([]string{"serve"}) || avero.Inspecting(nil) {
		t.Fatal("Inspecting = true for a command that is not an inspection")
	}
}
