package module

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/alternayte/avero/router"
)

// Set holds the modules of one application and the result of the inspection.
//
// Modules performs the inspection one time. A method of Set reads the result.
// A method never calls a module again.
type Set struct {
	rows    []Contribution
	probes  []*router.Router
	sched   Scheduler
	inbox   []InboxHandler
	proj    []Projection
	migs    []fs.FS
	faults  []*Fault
	byRoute map[string]routeOwner
}

// routeOwner names the module that registered one pattern.
type routeOwner struct {
	module string
	route  router.Route
}

// Contribution is one row of the contribution table. It states what one module
// contributes.
type Contribution struct {
	// Module is the name of the module.
	Module string `json:"module"`
	// Interfaces names each optional interface that the module implements,
	// in alphabetical order. It is empty for a module that implements none.
	Interfaces []string `json:"interfaces"`
	// Routes lists the routes of the module.
	Routes []RouteDesc `json:"routes"`
	// InboxHandlers names each message handler of the module.
	InboxHandlers []string `json:"inboxHandlers"`
	// Projections names each projection of the module.
	Projections []string `json:"projections"`
	// Jobs names each scheduled job of the module.
	Jobs []string `json:"jobs"`
	// Migrations reports a module that carries migration files.
	Migrations bool `json:"migrations"`
	// Description is the description of the module. The module system builds
	// it from the inspection when the module implements no DescribeModule.
	Description *Description `json:"description"`
}

// Modules inspects each module one time and returns the result.
//
// It calls each optional method one time, in the order of the arguments. It
// records a fault for a module with no name, for a name that two modules
// share, for a route registration that is wrong, and for one pattern that two
// modules register. Err returns the faults.
//
//	set := avero.Modules(billing.New(db), catalog.New(db))
//	if err := set.Attach(r); err != nil {
//	    os.Exit(avero.Exit(os.Stderr, err))
//	}
func Modules(ms ...Module) *Set {
	s := &Set{byRoute: make(map[string]routeOwner)}
	names := make(map[string]bool, len(ms))
	for i, m := range ms {
		s.inspect(i, m, names)
	}
	return s
}

// inspect reads one module.
func (s *Set) inspect(i int, m Module, names map[string]bool) {
	if m == nil {
		s.fault("", 0, fmt.Sprintf("the module at position %d is nil", i+1),
			"Delete the nil argument of Modules, or pass the module value")
		return
	}
	name := m.Name()
	row := Contribution{Module: name}
	probe := router.New()
	s.probes = append(s.probes, probe)

	switch {
	case name == "":
		s.fault("", 0, fmt.Sprintf("the module at position %d has an empty name", i+1),
			"Return a name from the Name method of the module")
	case names[name]:
		s.fault("", 0, fmt.Sprintf("two modules carry the name %q", name),
			"Give each module a name of its own")
	}
	names[name] = true

	if h, ok := m.(HTTPModule); ok {
		row.Interfaces = append(row.Interfaces, "HTTPModule")
		h.Routes(probe)
		s.readRoutes(name, probe, &row)
	}
	var types []string
	if h, ok := m.(InboxModule); ok {
		row.Interfaces = append(row.Interfaces, "InboxModule")
		for _, handler := range h.InboxHandlers() {
			s.inbox = append(s.inbox, handler)
			row.InboxHandlers = append(row.InboxHandlers, handler.HandlerName())
			types = append(types, handler.MessageType())
		}
	}
	if h, ok := m.(ScheduleModule); ok {
		row.Interfaces = append(row.Interfaces, "ScheduleModule")
		before := len(s.sched.jobs)
		h.Schedule(&s.sched)
		for _, job := range s.sched.jobs[before:] {
			row.Jobs = append(row.Jobs, job.Name)
		}
	}
	if h, ok := m.(ProjectorModule); ok {
		row.Interfaces = append(row.Interfaces, "ProjectorModule")
		for _, p := range h.Projections() {
			s.proj = append(s.proj, p)
			row.Projections = append(row.Projections, p.ProjectionName())
		}
	}
	if h, ok := m.(MigrationModule); ok {
		row.Interfaces = append(row.Interfaces, "MigrationModule")
		if files := h.Migrations(); files != nil {
			s.migs = append(s.migs, files)
			row.Migrations = true
		}
	}
	if h, ok := m.(DescribeModule); ok {
		row.Interfaces = append(row.Interfaces, "DescribeModule")
		d := h.Describe()
		if d.Name == "" {
			d.Name = name
		}
		if d.Routes == nil {
			d.Routes = row.Routes
		}
		row.Description = &d
	} else {
		row.Description = describe(row, types)
	}
	sort.Strings(row.Interfaces)
	row.Description.normalise()
	s.rows = append(s.rows, row)
}

// readRoutes records the routes of one module and finds a pattern that two
// modules share.
func (s *Set) readRoutes(name string, probe *router.Router, row *Contribution) {
	for _, f := range probe.Faults() {
		s.fault(f.File, f.Line, fmt.Sprintf("the module %q registers a route that is wrong: %s", name, f.Message),
			f.Repair)
	}
	for _, route := range probe.Routes() {
		key := route.Method + " " + route.Pattern
		if first, ok := s.byRoute[key]; ok {
			s.fault(route.File, route.Line,
				fmt.Sprintf("the modules %q and %q both register %s, which %s registers at %s:%d",
					first.module, name, key, first.module, first.route.File, first.route.Line),
				fmt.Sprintf("Delete the route from one of the two modules, or give one of them a pattern of its own, such as `%s/…`", route.Pattern))
			continue
		}
		s.byRoute[key] = routeOwner{module: name, route: route}
		row.Routes = append(row.Routes, RouteDesc{
			Method: route.Method, Pattern: route.Pattern, Handler: route.Handler,
		})
	}
}

// describe builds the description of a module that implements no
// DescribeModule. It states what the inspection found.
func describe(row Contribution, types []string) *Description {
	return &Description{
		Name:        row.Module,
		Routes:      row.Routes,
		Projections: row.Projections,
		InboxTypes:  types,
	}
}

// fault records one fault.
func (s *Set) fault(file string, line int, message, repair string) {
	s.faults = append(s.faults, &Fault{File: file, Line: line, Message: message, Repair: repair})
}

// Err returns every fault that the inspection found, or nil. The caller stops
// the process on a fault. See DX-8.
func (s *Set) Err() error {
	if len(s.faults) == 0 {
		return nil
	}
	return &Faults{Faults: s.faults}
}

// Attach adds the routes of every HTTPModule to r. It returns the faults and
// adds no route when the inspection found one.
//
// The prefix and the middleware of r apply to every route, so a group scopes a
// whole module.
func (s *Set) Attach(r *router.Router) error {
	if err := s.Err(); err != nil {
		return err
	}
	for _, probe := range s.probes {
		r.Merge(probe)
	}
	return nil
}

// Jobs returns the scheduled jobs of every ScheduleModule.
func (s *Set) Jobs() []Job { return s.sched.Jobs() }

// InboxHandlers returns the handlers of every InboxModule.
func (s *Set) InboxHandlers() []InboxHandler {
	out := make([]InboxHandler, len(s.inbox))
	copy(out, s.inbox)
	return out
}

// Projections returns the projections of every ProjectorModule.
func (s *Set) Projections() []Projection {
	out := make([]Projection, len(s.proj))
	copy(out, s.proj)
	return out
}

// Migrations returns the file system of every MigrationModule, in registration
// order.
func (s *Set) Migrations() []fs.FS {
	out := make([]fs.FS, len(s.migs))
	copy(out, s.migs)
	return out
}
