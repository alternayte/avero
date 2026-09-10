# Routing and handlers

Avero uses `net/http` and the pattern syntax of Go 1.22. It adds no third-party
router.

## A route

```go
func (m *Module) Routes(r *avero.Router) {
	avero.Get(r, "/posts", m.List, avero.Summary("List every post"))
	avero.Post(r, "/posts", m.Create, avero.Summary("Write one post"))
	avero.Get(r, "/posts/{id}", m.Show,
		avero.Summary("Answer one post"),
		avero.Answers[avero.Problem](http.StatusNotFound, "the post does not exist"))
}
```

The registration reads the type of the input and the type of the answer from
the handler. The compiler holds both, so a change of either is a fault of the
build and never a fault of a reader.

Write `/{$}` for the root page. A bare slash takes every path that no other
route holds, and it conflicts with a mounted handler.

A page of the ssr shape answers a view and names no body, so it registers with
`r.Get("/posts", avero.In(m.List))`.

`Group` returns a child scope with a prefix and its own middleware. `Mount`
serves an `http.Handler` under a prefix.

## A handler

```go
func (m *Module) Show(c *avero.Ctx, in ShowInput) (View, error) {
	post, found, err := m.store.Get(c, in.ID)
	if err != nil {
		return View{}, err
	}
	if !found {
		return View{}, avero.NotFound("post", in.ID)
	}
	return view(post), nil
}
```

A handler returns the thing that it answers, as an ordinary Go function does,
so a test calls the method and reads the value.

The method of the route states the status. A POST answers 201 and every other
method answers 200. Write `c.Status(code)` for another status. A handler that
answers no body returns `avero.NoBody`.

A handler never writes to the ResponseWriter, so the transaction commits before
one byte reaches the client.

The `Ctx` is a `context.Context`, so a handler passes `c` to a store, to a
generated client or to any function that takes a context. The value always
reads the context of the current request, so a value that a middleware adds
later is visible. `c.Context()` returns the plain context for a caller that
wants it. A `Ctx` is valid for one request only. Do not hold it after the
handler returns.

A page of the ssr shape answers a view instead, so it returns
`(avero.Response, error)` and renders with `avero.View`.

## The input type

```go
type CreateInput struct {
	// Title comes from the form or from the JSON body.
	Title string `form:"title" json:"title" validate:"required,min=3,max=80"`
	// ID comes from the path.
	ID string `path:"id" validate:"required"`
	// Page comes from the query.
	Page int `query:"page" validate:"min=1"`
}
```

`avero generate` writes `Bind` and `Validate` for each input type. The
generated file imports no reflect, and no request path reads a struct tag.

The sources are the JSON body, the form, the query and the path, in that order.
A later source replaces an earlier one, so the path wins.

## The rules

| Rule | Meaning |
|---|---|
| `required` | the value must be present |
| `min=3` | the length of a string, or the value of a number |
| `max=80` | the length of a string, or the value of a number |
| `email` | the value must be an email address |
| `uuid` | the value must be a UUID |
| `oneof=a b c` | the value must be one of the words |

Write a rule of your own as a `Check` method:

```go
func (in *CreateInput) Check(c *avero.Ctx, f *avero.Fields) {
	if in.Title == "admin" {
		f.Add("title", "must not be a reserved word")
	}
}
```

A message carries the name that the person sent, such as `title`, so a view
reads it with the name that it writes in the form.

## The responses

`Text`, `HTML`, `JSON`, `Redirect`, `NoContent`, `Empty`, `Status` and, for a
page, `View`. A validation fault answers 422 with a map of field name to
message, or the form again when the router carries `avero.WithForm`.

## The middleware

`avero.Stack` states the whole chain in one call. The order of the chain is a
correctness property, so Avero owns it and no application repeats it.

```go
r.Use(avero.Stack{
	Secret:  cfg.Secret,
	Logger:  logger,
	Trace:   provider.HTTPMiddleware(),
	Engine:  engine,
	Session: []avero.Middleware{avero.Adapt("session", auth.LoadSession)},
	Auth:    []avero.Middleware{avero.Adapt("auth", auth.RequireAuth)},
}.Middleware()...)
```

The order is request ID, recover, access log, trace, flash, session, CSRF,
transaction and authentication. A field with no value leaves its middleware
out, so an inspection command passes a nil engine and runs no transaction.

`Middleware` builds the chain of an application that serves a browser.
`API` builds the same chain without the flash cookie and without the CSRF
token, because a JSON client needs neither. The name states the choice, so a
page that needs the CSRF token cannot lose it through a forgotten field.

A person who wants another chain calls the middleware one at a time, as
`r.Use(avero.RequestID())` does.

The transaction middleware opens one transaction for each request. It reads the
status of the response and commits or rolls back. `avero.Adapt` turns a
`net/http` middleware into an Avero middleware, and it reports the status that
the middleware wrote, so a middleware that answers 500 rolls back.

## The description of the API

```
avero routes --openapi                       write the description to the output
avero routes --openapi --out openapi.json    write it to a file
avero routes --openapi --server https://api.example.com
```

The command reads the source, so it needs no database and no running
application. It reads the routes of each module and the input type of each
handler, and it writes an OpenAPI 3.1 document:

- a path for each pattern, with `{id}` as a path parameter;
- an operation for each method, named after the handler, with the first
  sentence of its comment as the summary;
- a parameter for each `path`, `query` and `header` field;
- a request body for each `json` field of a method that carries one;
- the rules of the `validate` tag as `minLength`, `maxLength`, `minimum`,
  `maximum`, `format` and `enum`;
- the answers that the router writes: 400 for a request that does not bind,
  422 for a request that fails validation, and 500 for a handler that returns
  an error.

The command runs the application with its inspection flag and reads the
operations that the router holds. A route registration touches no database and
opens no port, so the command needs no infrastructure. It reads no comment.

The type of each handler states the schema of its answer:

```go
func (m *Module) List(c *avero.Ctx, in ListInput) (TaskList, error)
```

The description holds one schema for each type that a handler names, and for
each type that such a type holds. A field is required, because Go writes every
field of a struct, unless its JSON tag carries omitempty.

An option states what a signature cannot carry:

The comment of a handler states its summary. `avero generate` writes the
sentence into the generated file, so a person writes it one time:

```go
// List answers every post, newest first.
func (m *Module) List(c *avero.Ctx, in ListInput) (PostList, error)
```

The description then reads "Answers every post, newest first". The name of the
method leaves the sentence, because the description names the operation beside
its summary. An option of a route wins over the comment.

| Option | Meaning |
|---|---|
| `avero.Summary("List every post")` | the line that a person reads, over the comment |
| `avero.Describe("...")` | the paragraph, where a summary is one line |
| `avero.Tags("posts")` | the group of the operation |
| `avero.Deprecated()` | the operation that a caller must leave |
| `avero.Answers[T](code, "why")` | one more status, with the type of its body |
| `avero.AnswersNothing(code, "why")` | one more status, with no body |
| `avero.AnswerHeader(201, "Location", "...")` | a header that one answer carries |
| `avero.Secured("bearer")` | the scheme that the route needs |
| `avero.Public()` | the route that needs no identity |

Two calls of `avero.Answers` with one status state an answer that carries one
of two shapes. The description writes `oneOf`.

Every route states its faults as a problem document against one Problem schema.
A route that binds a value states 400 and 422. See [Errors](errors.md).

## The prose of a model

A tag states what a member means, and what a reader recognises:

```go
type View struct {
	// Title is the name that a person reads.
	Title string `json:"title" doc:"The name that a person reads" example:"A title"`
	// Rank orders the post.
	Rank *int `json:"rank" openapi:"readOnly" default:"0"`
}
```

| Tag | Meaning |
|---|---|
| `doc:"..."` | the description of the member |
| `example:"..."` | one value that a reader recognises |
| `default:"..."` | the value that the service uses when the request carries none |
| `format:"..."` | the format, where the rule of the field states none |
| `pattern:"..."` | the regular expression that a string must match |
| `openapi:"readOnly"` | the member that only an answer carries |
| `openapi:"writeOnly"` | the member that only a request carries |
| `openapi:"deprecated"` | the member that a caller must leave |
| `openapi:"uniqueItems"` | the array that holds each value one time |

A number in a tag reaches the document as a number.

A model states its own sentence with a method, because a type carries no tag:

```go
// Doc states what the model means.
func (View) Doc() string { return "One post as the API returns it" }
```

## The facts of the whole API

```go
r := avero.NewRouter(avero.WithAPI(avero.API{
	Title:       "Blog",
	Description: "The service that holds the posts.",
	Servers:     []avero.Server{{URL: "https://api.example.com", Description: "production"}},
	Security:    map[string]avero.SecurityScheme{"bearer": avero.BearerAuth("The token of a session.")},
	Require:     []string{"bearer"},
	Tags:        []avero.Tag{{Name: "posts", Description: "The posts of the blog."}},
}))
```

The block carries the name, the version, the prose, the terms, the contact, the
licence, the addresses, the schemes of the identity, the groups and the
document that stands outside. `avero.BearerAuth`, `avero.BasicAuth` and
`avero.APIKeyAuth` write a scheme.

## The types that a description carries

| Go | The document |
|---|---|
| `time.Time` | `string`, `date-time` |
| `time.Duration` | `integer`, `int64` |
| `uuid.UUID` | `string`, `uuid` |
| `[]byte` | `string`, `byte` |
| `map[string]T` | an object, with the schema of T |
| `*T` that an answer carries | the type of T, and `null` |
| a type with `MarshalText` | `string` |
| a struct | a reference, and the components hold it |

The media type of a request comes from the tags of its input. A field with a
`json` tag reads JSON, and a field with a `form` tag reads a form.

## The modules

```go
modules := avero.Modules(posts.New(engine), comments.New(engine))
if err := modules.Attach(r); err != nil {
	return err
}
```

A module states a name and implements the interfaces of the concerns that it
carries: `Routes`, `InboxHandlers`, `Schedule`, `Projections`, `Migrations` and
`Describe`. `avero generate` writes a `Models` method from the `model` package
of the feature slice, so a module states no hand-written `Describe` method.
Avero inspects each module one time at start. Two modules that register one
pattern stop the process, and the fault names both modules.
