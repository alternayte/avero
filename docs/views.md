# Views and forms

The view package holds the helpers that a page needs. The `internal/ui` package
of your application holds the pages. It imports no feature package.

## A page

The ssr shape writes its pages as templ files:

```templ
package ui

templ PostList(rows []PostRow) {
	<h1>Posts</h1>
	<ul class="posts">
		for _, row := range rows {
			<li><a href={ templ.URL("/posts/" + row.ID) }>{ row.Title }</a></li>
		}
	</ul>
}
```

`go tool templ generate` writes the Go file of each templ file. The
application carries the generator in its `go.mod`, so a person installs
nothing. `avero generate`, `avero build` and `avero dev` run it.

A component states one method:

```go
Render(ctx context.Context, w io.Writer) error
```

A templ component satisfies it, and so does `view.Func`. Avero imports no
template library, so an application can use templ, `html/template` or a plain
function.

## The response

```go
return avero.View(ui.Page("Posts", ui.PostList(rows))), nil
return avero.ViewStatus(http.StatusNotFound, ui.Page("Absent", ui.NotFound())), nil
```

`View` renders into a buffer first. A render fault therefore reaches the error
response, and no half page reaches the client.

## The form helpers

| Helper | Answer |
|---|---|
| `view.Old(ctx, "title")` | the value that the person submitted |
| `view.Error(ctx, "title")` | the validation message of the field |
| `view.HasError(ctx, "title")` | whether the field failed |
| `view.Toasts(ctx)` | the messages of this response |
| `view.CSRF()` | the hidden field that an unsafe form must carry |

Each helper answers for a request that submitted nothing, so a first render
needs no test.

## The form again

```go
r := avero.NewRouter(avero.WithForm(func(c *avero.Ctx, f *avero.Fields) avero.ViewComponent {
	return ui.Page("New post", ui.NewPostForm())
}))
```

A validation fault answers 422 and renders the page again. The render context
holds the field errors and the submitted values.

## The toasts

```go
c.Success("The post is saved")
return avero.Redirect(http.StatusSeeOther, "/"), nil
```

The flash middleware carries a toast across a redirect in a signed cookie. The
next page reads it with `view.Toasts`. A toast is read one time.

## The CSRF token

The CSRF middleware gives a safe method a signed token in a cookie. An unsafe
method must send the same token in the field `_csrf` or in the header
`X-CSRF-Token`. A request that does not match answers 419 and never reaches the
handler.

## A fragment

`ds` and `htmx` render the same components into a fragment or into a stream.
Both packages are optional imports, and the `ui` package imports neither.

```go
// Datastar patches the element into the page.
return ds.Open(func(s *ds.Stream) error {
	return s.PatchElements(ui.PostList(rows))
}), nil

// htmx answers one fragment.
return htmx.Fragment(ui.PostList(rows)), nil
```

A component reads `ds.Partial(c)` or `htmx.Partial(c)` and renders a fragment
for a partial request and the whole page for any other.
