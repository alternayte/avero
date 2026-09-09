package router

// API states the facts of the whole description of an API.
//
// A route states its own operation. This states what stands above every route:
// the name, the prose, the addresses, the schemes of the identity and the
// groups.
//
//	r := avero.NewRouter(avero.WithAPI(avero.API{
//	    Title:       "Blog",
//	    Description: "The service that holds the posts.",
//	    Servers: []Server{{URL: "https://api.example.com", Description: "production"}},
//	    Security: map[string]SecurityScheme{
//	        "bearer": avero.BearerAuth("The token of a session."),
//	    },
//	    Require: []string{"bearer"},
//	}))
type API struct {
	// Title is the name of the API. An empty value reads the name of the
	// module of the binary.
	Title string
	// Version is the version of the description. An empty value reads the
	// version of the binary.
	Version string
	// Description states what the API does. A reader of the document reads
	// it first.
	Description string
	// TermsOfService is the address of the terms.
	TermsOfService string
	// Contact names the people who own the API.
	Contact *Contact
	// License names the licence of the API.
	License *License
	// Servers name the addresses that answer.
	Servers []Server
	// Security holds the schemes of the identity, by name.
	Security map[string]SecurityScheme
	// Require names the schemes that every route needs. A route states
	// Public to stand outside them.
	Require []string
	// Tags describe the groups that the operations name.
	Tags []Tag
	// ExternalDocs names a document that stands outside this one.
	ExternalDocs *ExternalDocs
}

// Contact names the people who own the API.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License names the licence of the API.
type License struct {
	Name string `json:"name"`
	// Identifier is the SPDX name, such as MIT.
	Identifier string `json:"identifier,omitempty"`
	URL        string `json:"url,omitempty"`
}

// Server is one address that answers.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// Tag describes one group of operations.
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ExternalDocs names a document that stands outside this one.
type ExternalDocs struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// SecurityScheme states how a caller proves its identity.
type SecurityScheme struct {
	// Type is http, apiKey, oauth2 or openIdConnect.
	Type string `json:"type"`
	// Description states the scheme for a person.
	Description string `json:"description,omitempty"`
	// Scheme is the name of an HTTP scheme, such as bearer or basic.
	Scheme string `json:"scheme,omitempty"`
	// BearerFormat names the shape of a bearer token, such as JWT.
	BearerFormat string `json:"bearerFormat,omitempty"`
	// Name is the name of the header, the query member or the cookie of an
	// apiKey scheme.
	Name string `json:"name,omitempty"`
	// In is header, query or cookie, for an apiKey scheme.
	In string `json:"in,omitempty"`
	// OpenIDConnectURL names the document of an openIdConnect scheme.
	OpenIDConnectURL string `json:"openIdConnectUrl,omitempty"`
}

// BearerAuth returns the scheme of a bearer token.
func BearerAuth(description string) SecurityScheme {
	return SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT", Description: description}
}

// BasicAuth returns the scheme of a name and a password.
func BasicAuth(description string) SecurityScheme {
	return SecurityScheme{Type: "http", Scheme: "basic", Description: description}
}

// APIKeyAuth returns the scheme of a key that a header carries.
func APIKeyAuth(header, description string) SecurityScheme {
	return SecurityScheme{Type: "apiKey", In: "header", Name: header, Description: description}
}

// WithAPI states the facts of the whole description of the API.
func WithAPI(api API) Option {
	return func(r *Router) { r.reg.api = api }
}

// API returns the facts that WithAPI stated.
func (r *Router) API() API { return r.reg.api }
