# Errors

A service answers one error shape. Avero writes it, so a feature states the
case and not the JSON.

## Answer a failure

Return the problem. The router writes it.

```go
func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Result[View], error) {
	post, found, err := m.store.Get(c.Context(), in.ID)
	if err != nil {
		return avero.Result[View]{}, err
	}
	if !found {
		return avero.Result[View]{}, avero.NotFound("post", in.ID)
	}
	return avero.OK(view(post)), nil
}
```

The client reads a problem document, which RFC 9457 states:

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "the post 4f1b does not exist",
  "instance": "/posts/4f1b"
}
```

The media type is `application/problem+json`.

## The helpers

| Helper | Status |
|---|---|
| `avero.NotFound(thing, id)` | 404 |
| `avero.BadRequest(detail)` | 400 |
| `avero.Unauthorized(detail)` | 401 |
| `avero.Forbidden(detail)` | 403 |
| `avero.Conflict(detail)` | 409 |
| `avero.NewProblem(code, detail)` | the code that you name |

## An error of the service

An error that carries no problem is a fault of the service. The answer is 500,
and the message stays in the log. A message can name a table, a host or a
query, and a person outside the service reads none of it.

```go
return avero.Result[View]{}, fmt.Errorf("the post does not read: %w", err)
```

## Keep the cause

A problem wraps the error that a store returned. The log reads the cause, and
the client does not.

```go
return avero.Result[View]{}, avero.Conflict("the title is taken").Wrap(err)
```

`errors.Is` and `errors.As` reach both:

```go
if errors.Is(err, avero.ErrNotFound) { ... }
if errors.Is(err, sql.ErrNoRows) { ... }
```

`errors.Is` compares the status, so `avero.ErrConflict` answers for every 409.

## Add a member

RFC 9457 allows a member beside the standard ones. A validation fault uses it
for the field messages.

```go
return avero.Result[View]{}, avero.Conflict("the post is locked").
	With("lockedBy", name)
```

## The validation fault

`avero generate` writes the check of each input type. A field that fails
answers 422 with one message for each field:

```json
{
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "one field or more failed validation",
  "errors": {"title": "the title needs 3 characters or more"}
}
```

The ssr shape answers the form again instead, with the old input and the field
errors. See [Views and forms](views.md).

## The description of the API

Every route states its faults against one Problem schema, so a client holds one
error type. A route that binds a value states 400 and 422. A route that answers
a further case states it at the registration:

```go
avero.Get(r, "/posts/{id}", m.Show,
	avero.Answers[avero.Problem](http.StatusNotFound, "the post does not exist"))
```

## Change the answer

`avero.WithErrorResponse` replaces the writer of an error for the whole router.
The default reads the problem that the error carries.
