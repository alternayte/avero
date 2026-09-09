# The single page shape

`avero new board --shape spa` writes a Go service and a TypeScript front end.
Vite builds the front end, and the Go binary carries it.

```
board/
├── main.go                        the composition root
├── wire.go                        the router and the embedded build
├── internal/features/tasks/       the JSON routes, the handlers and the store
├── web/                           the front end
│   ├── package.json               the dependencies of the front end
│   ├── vite.config.ts             the build and the proxy of the API
│   ├── tsconfig.json              strict TypeScript
│   ├── index.html                 the document that Vite writes again
│   └── src/
│       ├── main.tsx               the root of the front end
│       ├── Board.tsx              the page
│       ├── api.ts                 the client of the JSON API
│       └── styles.css             Tailwind
├── assets/dist/                   the build. `avero build` writes it.
├── migrations/                    the SQL files
└── avero.json                     the shape and the bundler
```

## What ships

React 19, TanStack Query, TypeScript, Tailwind and Vite. The shape needs
Node.js, and it uses it where it earns its place: the dependency tree, the type
check and the hot module replacement of the development server.

The other shapes need no Node.js. See [Assets](assets.md).

## Run it

```
cp .env.example .env
avero migrate up
avero dev
```

`avero dev` starts three things:

- the Go application on a port of its own;
- `npm run dev`, which is Vite, on http://localhost:5173;
- the watcher, which rebuilds and restarts the Go application on a change to a
  `.go` file.

Open the address of Vite. It replaces a module in the page with no reload, and
it proxies `/api` to the Go application, so one origin serves both in
development. `avero dev` gives Vite the address of the application in
`AVERO_API_URL`, so the port never has to match by hand.

`avero dev` reads the dependencies with `npm install` when `web/node_modules`
is absent, so one command starts a front end that a person just wrote.

## Add a JavaScript library

```
cd web
npm install @tanstack/react-table
```

Import it by its name:

```tsx
import { useReactTable } from "@tanstack/react-table";
```

Vite picks it up at once. `avero build` reads the same dependencies, so a
colleague and the build machine get the versions that `package-lock.json`
states.

## Call the API

`web/src/api.ts` holds the types of the answers and one function for each call.
`Board.tsx` reads them with TanStack Query:

```tsx
const tasks = useQuery({ queryKey: ["tasks"], queryFn: api.listTasks });

const create = useMutation({
    mutationFn: api.createTask,
    onError: (error: unknown) => {
        // The Go handler answers 422 with one message for each field.
        setFault(error instanceof RequestFault ? error.fields.title : "the task does not save");
    },
});
```

The Go side states the same shape:

```go
type CreateInput struct {
	Title string `json:"title" validate:"required,min=1,max=200"`
}
```

`avero routes --openapi` writes the description of the API, which a generator
reads for a larger service. See [Routing and handlers](routing.md).

## One binary

`avero build` runs `npm install`, then `tsc --noEmit && vite build`, which
writes `assets/dist`. `wire.go` embeds that directory:

```go
//go:embed all:assets/dist
var dist embed.FS

files, _ := fs.Sub(dist, "assets/dist")
r.Mount("/", assets.SPA(files))
```

The handler answers a file of the build, with a cache of one year for a hashed
name, and it answers the index document for every path that the front end owns,
so a deep link and a reload work.

`main.go` embeds the migrations as well, so the binary carries the schema:

```go
//go:embed all:migrations
var migrations embed.FS
```

One file therefore holds the server, the front end and the schema:

```dockerfile
FROM gcr.io/distroless/static-debian12
COPY bin/board /board
ENV MIGRATE_ON_BOOT=true
ENTRYPOINT ["/board"]
```

```
avero build
docker build -t board .
docker run -e AVERO_SECRET=… -e DATABASE_URL=… -p 8080:8080 board
```

## Add a page

The shape ships one page. For several, add a router:

```
cd web && npm install @tanstack/react-router
```

The server already answers the index document for every path that no route
holds, so a reload of a deep link reaches the front end.
