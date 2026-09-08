package host_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/host"
)

func check(name string, err error) host.Check {
	return host.Check{
		Name:   name,
		Repair: "Start " + name + " and run the application again",
		Run:    func(context.Context) error { return err },
	}
}

func TestABootCheckFaultStopsTheProcessBeforeStart(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(time.Second),
		host.WithComponents(newFake(rec, "a")),
		host.WithChecks(check("the database", errFake)))

	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run started although a boot check failed")
	}
	rec.equal(t)
}

func TestEveryBootCheckFaultIsReported(t *testing.T) {
	app := newApp(t, baseConfig(time.Second), host.WithChecks(
		check("the database", errFake),
		check("the broker", nil),
		check("the migrations", errFake),
	))

	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run started although a boot check failed")
	}
	var faults *host.BootFaults
	if !errors.As(err, &faults) {
		t.Fatalf("err is %T, want *host.BootFaults", err)
	}
	if len(faults.Faults) != 2 {
		t.Fatalf("the loader reported %d faults, want 2: %v", len(faults.Faults), err)
	}
	for _, name := range []string{"the database", "the migrations"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("the message does not name %q: %v", name, err)
		}
	}
	if strings.Contains(err.Error(), "the broker") {
		t.Fatalf("the message names a check that passed: %v", err)
	}
}

func TestEveryBootFaultStatesTheRepair(t *testing.T) {
	app := newApp(t, baseConfig(time.Second), host.WithChecks(check("the database", errFake)))
	err := runOnce(t, app)
	var faults *host.BootFaults
	if !errors.As(err, &faults) {
		t.Fatalf("err is %T, want *host.BootFaults", err)
	}
	for _, f := range faults.Faults {
		if f.Repair == "" {
			t.Fatalf("the fault for %q states no repair", f.Check)
		}
		if !strings.Contains(f.Error(), f.Repair) {
			t.Fatalf("the message for %q drops the repair: %q", f.Check, f.Error())
		}
		if !errors.Is(f, errFake) {
			t.Fatalf("the fault for %q does not wrap its cause", f.Check)
		}
	}
}

func TestABootCheckWithNoRepairIsAFault(t *testing.T) {
	app := newApp(t, baseConfig(time.Second), host.WithChecks(host.Check{
		Name: "the database",
		Run:  func(context.Context) error { return nil },
	}))
	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run accepted a boot check that states no repair")
	}
	if !strings.Contains(err.Error(), "Repair") {
		t.Fatalf("the message does not name the missing field: %v", err)
	}
}

func TestABootCheckWithNoNameIsAFault(t *testing.T) {
	app := newApp(t, baseConfig(time.Second), host.WithChecks(host.Check{
		Repair: "Set it",
		Run:    func(context.Context) error { return nil },
	}))
	if err := runOnce(t, app); err == nil {
		t.Fatal("Run accepted a boot check with no name")
	}
}

func TestABootCheckWithNoRunIsAFault(t *testing.T) {
	app := newApp(t, baseConfig(time.Second), host.WithChecks(host.Check{
		Name:   "the database",
		Repair: "Set it",
	}))
	if err := runOnce(t, app); err == nil {
		t.Fatal("Run accepted a boot check with no Run")
	}
}

func TestBootChecksRunInRegistrationOrder(t *testing.T) {
	rec := &recorder{}
	order := func(name string) host.Check {
		return host.Check{
			Name:   name,
			Repair: "Repair " + name,
			Run:    func(context.Context) error { rec.add("check " + name); return nil },
		}
	}
	app := newApp(t, baseConfig(time.Second), host.WithChecks(order("a"), order("b"), order("c")))
	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	rec.equal(t, "check a", "check b", "check c")
}

func TestAComponentWithNoNameIsAFault(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(time.Second), host.WithComponents(newFake(rec, "")))
	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run accepted a component with no name")
	}
	rec.equal(t)
}

func TestTwoComponentsWithTheSameNameIsAFault(t *testing.T) {
	rec := &recorder{}
	app := newApp(t, baseConfig(time.Second),
		host.WithComponents(newFake(rec, "db"), newFake(rec, "broker"), newFake(rec, "db")))
	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run accepted two components with the same name")
	}
	if !strings.Contains(err.Error(), "db") {
		t.Fatalf("the message does not name the component: %v", err)
	}
	rec.equal(t)
}

func TestTheMigratorRunsWhenMigrateOnBootIsTrue(t *testing.T) {
	rec := &recorder{}
	cfg := baseConfig(time.Second)
	cfg.MigrateOnBoot = true
	app := newApp(t, cfg,
		host.WithMigrator(&migrator{rec: rec}),
		host.WithComponents(newFake(rec, "a")))

	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	// The migration runs after the boot checks and before the first Start.
	rec.equal(t, "migrate", "start a", "stop a")
}

func TestTheMigratorDoesNotRunWhenMigrateOnBootIsFalse(t *testing.T) {
	rec := &recorder{}
	cfg := baseConfig(time.Second)
	cfg.MigrateOnBoot = false
	app := newApp(t, cfg,
		host.WithMigrator(&migrator{rec: rec}),
		host.WithComponents(newFake(rec, "a")))

	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	rec.equal(t, "start a", "stop a")
}

func TestAMigrationFaultStopsTheProcessBeforeStart(t *testing.T) {
	rec := &recorder{}
	cfg := baseConfig(time.Second)
	cfg.MigrateOnBoot = true
	app := newApp(t, cfg,
		host.WithMigrator(&migrator{rec: rec, err: errFake}),
		host.WithComponents(newFake(rec, "a")))

	err := runOnce(t, app)
	if err == nil {
		t.Fatal("Run started although the migration failed")
	}
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("the error states no repair: %v", err)
	}
	rec.equal(t, "migrate")
}

func TestABootCheckRunsBeforeTheMigrator(t *testing.T) {
	rec := &recorder{}
	cfg := baseConfig(time.Second)
	cfg.MigrateOnBoot = true
	app := newApp(t, cfg,
		host.WithMigrator(&migrator{rec: rec}),
		host.WithChecks(host.Check{
			Name:   "the database",
			Repair: "Start the database",
			Run:    func(context.Context) error { rec.add("check"); return nil },
		}))

	stop := run(t, app)
	waitReady(t, app)
	if err := stop(); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	rec.equal(t, "check", "migrate")
}
