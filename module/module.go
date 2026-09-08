// Package module holds the module contract and the module inspection, S6.
//
// A module is one feature of an application. It states a name. It implements
// the optional interface of each concern that it contributes to: routes,
// inbox handlers, scheduled jobs, projections, migrations and the description
// that an agent reads.
//
// Avero inspects each module one time at start. Avero does not use init for
// discovery. See the SDD, section 5.2 and S6.
package module

import (
	"context"
	"io/fs"

	"github.com/alternayte/avero/router"
)

// Module is one feature of an application. Every module states a name. The
// name appears in the contribution table and in a fault message, so it must
// stay stable.
type Module interface {
	Name() string
}

// HTTPModule contributes routes. The module registers them on the router that
// the module system passes to it.
type HTTPModule interface {
	Routes(r *router.Router)
}

// InboxModule contributes message handlers. S8 runs them.
type InboxModule interface {
	InboxHandlers() []InboxHandler
}

// InboxHandler is one message handler. The module system reads the name of the
// handler and the type of the message. S8 defines the concrete handler, and it
// satisfies this interface.
type InboxHandler interface {
	// HandlerName returns the name of the handler. The name and the message
	// identifier form the dedupe key.
	HandlerName() string
	// MessageType returns the type of the message that the handler reads.
	MessageType() string
}

// ScheduleModule contributes scheduled jobs.
type ScheduleModule interface {
	Schedule(s *Scheduler)
}

// ProjectorModule contributes projections. S9 runs them.
type ProjectorModule interface {
	Projections() []Projection
}

// Projection is one read model projection. The module system reads the name.
// S9 defines the concrete projection, and it satisfies this interface.
type Projection interface {
	// ProjectionName returns the name of the projection. The checkpoint row
	// carries it.
	ProjectionName() string
}

// MigrationModule contributes migration files. The file system holds the SQL
// files of one module.
type MigrationModule interface {
	Migrations() fs.FS
}

// DescribeModule states the description that an agent reads. A module that
// does not implement it still gets a description, which the module system
// builds from the inspection. See AN-3.
type DescribeModule interface {
	Describe() Description
}

// Scheduler collects the jobs of the modules. A ScheduleModule adds its jobs
// to it. The runner of the jobs arrives with a later subsystem.
type Scheduler struct {
	jobs []Job
}

// Job is one scheduled unit of work.
type Job struct {
	// Name identifies the job. It appears in the contribution table.
	Name string
	// Every states the period, such as 1m. A runner reads it.
	Every string
	// Run performs the work.
	Run func(ctx context.Context) error
}

// Add records one job.
func (s *Scheduler) Add(j Job) { s.jobs = append(s.jobs, j) }

// Jobs returns the recorded jobs in registration order.
func (s *Scheduler) Jobs() []Job {
	out := make([]Job, len(s.jobs))
	copy(out, s.jobs)
	return out
}
