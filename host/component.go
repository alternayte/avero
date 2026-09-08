package host

import "context"

// Component is a part of the application that has a lifecycle. Avero starts
// components in registration order and stops them in reverse order. See the
// SDD, section 5.1.
type Component interface {
	// Name identifies the component in a log line and in a fault.
	Name() string
	// Start brings the component up. A Start that returns an error stops the
	// run sequence. A panic in Start is a fault. Avero does not recover it.
	Start(ctx context.Context) error
	// Stop brings the component down. The context carries the SHUTDOWN_GRACE
	// deadline.
	Stop(ctx context.Context) error
}

// Ready is the optional readiness contract. A component that implements it
// gates /readyz. The endpoint answers 200 only when every implementation
// returns nil. See the SDD, section 5.1.
type Ready interface {
	Ready(ctx context.Context) error
}

// Check is one boot check. Avero runs every check before it starts the first
// component, so that a fault appears before the process serves. See DX-8.
//
// Name and Repair are mandatory. Avero refuses to run an application that
// carries a check with an empty field, because an error must state its repair.
// See DX-7.
type Check struct {
	// Name states what the check proves, such as "the database".
	Name string
	// Repair is one sentence that says what to do when the check fails.
	Repair string
	// Run performs the check. It returns nil when the check passes.
	Run func(ctx context.Context) error
}

// Migrator applies the pending migrations. Avero calls it at boot when
// MIGRATE_ON_BOOT is true. See the SDD, section 5.3, step 3.
type Migrator interface {
	Migrate(ctx context.Context) error
}
