# The single page shape

`avero new board --shape spa` writes a Go service with a React front end. One
binary holds both.

```
board/
├── main.go                        the composition root
├── wire.go                        the router and the embedded assets
├── internal/features/tasks/       the JSON routes, the handlers and the store
├── internal/ui/ui.go              the document that loads the front end
├── assets/js/app.jsx              the front end
├── assets/js/vendor/              the pinned modules
├── assets/css/app.css             the stylesheet
├── assets/dist/                   the output of `avero build`
└── avero.json                     the entry points of the pipeline
```

## What ships

React 19 and TanStack Query, vendored. The machine needs Go and no Node.js.
esbuild transforms the JSX with the automatic runtime, so a component file
needs no import of React, and it writes one bundle of about 230 kilobytes.

## Run it

```
cp .env.example .env
avero migrate up
avero dev
```

`avero dev` watches the tree:

| Change | Answer | Time |
|---|---|---|
| `assets/js/*.jsx` | rebuild the bundle, load the page again | about 60 ms |
| `assets/css/*.css` | rebuild the stylesheet, swap the link element | about 40 ms |
| a `.go` file | rebuild the binary, start it again, morph the page | under 2 s |

## Add a JavaScript library

One command. It names the package, and nothing else.

```
avero js pin zustand
```

The command asks the CDN for the package, fetches the bundled module, writes it
into `assets/js/vendor/`, and records the resolved address and the hash in
`avero.lock`. Import it by its name:

```jsx
import { create } from "zustand";
```

Then run `avero build`, or leave `avero dev` running, which rebuilds the bundle
on the next save.

More forms:

```
avero js pin zustand@5.0.15                 one version
avero js pin @tanstack/react-table          a scoped package
avero js pin react-dom/server               one subpath of a package
avero js pin lodash-es https://…/lodash.js  one address that you choose
```

A second run of the same pin changes nothing. A body that does not match the
hash of the lock stops the command, so a change at the CDN never reaches a
build without a person.

The vendor directory belongs in the repository, so a build needs no network and
a colleague reads the same bytes.

## Call the API

The front end reads the JSON routes of the application. TanStack Query holds
the cache and the state of each request:

```jsx
const tasks = useQuery({
    queryKey: ["tasks"],
    queryFn: () => read("/api/tasks"),
});

const create = useMutation({
    mutationFn: (title) => read("/api/tasks", { method: "POST", body: JSON.stringify({ title }) }),
    onSuccess: () => queries.invalidateQueries({ queryKey: ["tasks"] }),
});
```

The routes stand in `internal/features/tasks/module.go`, and the input types
carry their validation:

```go
type CreateInput struct {
	Title string `json:"title" validate:"required,min=1,max=200"`
}
```

An invalid body answers 422 with one message for each field, so the front end
reads `errors.title` and shows it.

## One binary

`wire.go` embeds the output directory:

```go
//go:embed all:assets/dist
var dist embed.FS
```

```
avero build
./bin/board
```

The binary serves the shell, the bundle and the stylesheet from its own file
system. It runs in a directory that holds nothing else, so a container carries
one file:

```dockerfile
FROM gcr.io/distroless/static-debian12
COPY bin/board /board
ENTRYPOINT ["/board"]
```

## Add a page

The shape ships one page. For several, pin a router and read the path in the
front end:

```
avero js pin @tanstack/react-router
```

The server answers the shell for the root path. Add one route for each path
that the front end owns, so a reload of a deep link reaches the same document:

```go
r.Get("/tasks/{id}", func(c *avero.Ctx) (avero.Response, error) {
	return avero.View(ui.Shell()), nil
})
```
