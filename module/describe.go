package module

// Description states what one module contributes. `avero modules --json`
// emits it, and the MCP server of S16 returns it. See AN-3.
type Description struct {
	// Name is the name of the module.
	Name string `json:"name"`
	// Routes lists the routes of the module.
	Routes []RouteDesc `json:"routes"`
	// Models lists the persistent models of the module.
	Models []ModelDesc `json:"models"`
	// Events lists the events that the module publishes.
	Events []EventDesc `json:"events"`
	// Projections names each projection of the module.
	Projections []string `json:"projections"`
	// InboxTypes names each message type that the module reads.
	InboxTypes []string `json:"inboxTypes"`
}

// RouteDesc is one route of a module.
type RouteDesc struct {
	// Method is the HTTP method, or * for a mounted handler.
	Method string `json:"method"`
	// Pattern is the pattern that the module registered.
	Pattern string `json:"pattern"`
	// Handler is the name of the function that serves the route.
	Handler string `json:"handler"`
}

// ModelDesc is one persistent model of a module.
type ModelDesc struct {
	// Name is the name of the Go type.
	Name string `json:"name"`
	// Table is the name of the database table.
	Table string `json:"table"`
	// Fields lists the fields of the model.
	Fields []FieldDesc `json:"fields"`
}

// EventDesc is one event of a module.
type EventDesc struct {
	// Name is the name of the event type.
	Name string `json:"name"`
	// Fields lists the fields of the event.
	Fields []FieldDesc `json:"fields"`
}

// FieldDesc is one field of a model or of an event.
type FieldDesc struct {
	// Name is the name of the field.
	Name string `json:"name"`
	// Type is the Go type of the field.
	Type string `json:"type"`
}

// normalise replaces a nil slice with an empty slice, so that the JSON
// document holds an array in every member. See AN-4.
func (d *Description) normalise() {
	if d.Routes == nil {
		d.Routes = []RouteDesc{}
	}
	if d.Models == nil {
		d.Models = []ModelDesc{}
	}
	if d.Events == nil {
		d.Events = []EventDesc{}
	}
	if d.Projections == nil {
		d.Projections = []string{}
	}
	if d.InboxTypes == nil {
		d.InboxTypes = []string{}
	}
	for i := range d.Models {
		if d.Models[i].Fields == nil {
			d.Models[i].Fields = []FieldDesc{}
		}
	}
	for i := range d.Events {
		if d.Events[i].Fields == nil {
			d.Events[i].Fields = []FieldDesc{}
		}
	}
}
