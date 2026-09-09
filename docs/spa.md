# The single page shape

`avero new board --shape spa` writes a Go service and a TypeScript front end.
Vite builds the front end, and the Go binary carries it.

```
board/
├── main.go                        the composition root
├── wire.go                        the router and the embedded build
├── internal/features/tasks/       the JSON routes, the handlers and the store
│   ├── model/                     the rows that drel reads
│   └── migrations/                the SQL of this feature. drel writes it.
├── web/                           the front end
│   ├── package.json               the dependencies of the front end
│   ├── vite.config.ts             the build and the proxy of the API
│   ├── tsconfig.json              strict TypeScript
│   ├── openapi-ts.config.ts       the generator of the client
│   ├── index.html                 the document that Vite writes again
│   └── src/
│       ├── main.tsx               the root of the front end
│       ├── Board.tsx              the page
│       ├── client/                the generated client. Never edit it.
│       └── styles.css             Tailwind
├── openapi.json                   the description that `avero build` writes
├── assets/dist/                   the build. `avero build` writes it.
├── drel.yaml                      the model packages and the migrations of drel
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

No shape is written two times. The Go handlers state the routes, the input
types and the answers. `avero build` and `avero dev` write the description of
the API, and the generator writes the client of the front end from it:

```
Go handler  →  openapi.json  →  web/src/client/  →  Board.tsx
```

The handler states its answer with a directive:

```go
// List answers every task.
//
//avero:response 200 TaskList
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	return avero.JSON(http.StatusOK, out), nil
}
```

The generator writes the types, the calls and the TanStack Query options:

```tsx
import { tasksListOptions, tasksCreateMutation } from "./client/@tanstack/react-query.gen";
import type { Task } from "./client";

const tasks = useQuery(tasksListOptions());
const create = useMutation({ ...tasksCreateMutation() });

create.mutate({ body: { title } });
```

`tasks.data` holds `TaskList`, and `task.title` is a `string`, because the Go
struct states it. A change to a handler that a component does not follow is a
type fault of the build, not a fault of a person reading a page.

Never edit `web/src/client/`. Change the Go handler and run `avero build`, or
`npm run api` inside `web/`.

### The generator

The shape uses [Hey API](https://heyapi.dev) with its TanStack Query plugin:

```ts
export default defineConfig({
    input: "../openapi.json",
    output: { path: "src/client", format: "prettier" },
    plugins: [
        "@hey-api/client-fetch",
        { name: "@tanstack/react-query", queryOptions: true, mutationOptions: true },
    ],
});
```

Hey API stands beside Orval, which does the same work. Hey API writes a compact
client with one call shape, `{ path, query, body }`, and it gives query options
and mutation options that a person composes with `useQuery` and `useMutation`.
Orval writes a whole hook for each route, and it writes mocks and validation
schemas as well. Avero states that a person reads, changes and deletes
generated code, and that explicit beats short, so the compact and composable
output fits. Read the trade-off in the sources at the end of this page.

The description also carries the faults that the router writes, so the client
types the 422 answer with its field messages.

## The description of the API

```
avero routes --openapi                    write it to the output
avero routes --openapi --out openapi.json write it to a file
```

`avero build` writes it before the front end build. See [Routing and
handlers](routing.md).

## One binary

`avero build` runs `npm install`, writes `openapi.json`, generates the client,
then runs `tsc --noEmit && vite build`, which writes `assets/dist`. `wire.go` embeds that directory:

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

## Sources

- [Hey API, the TanStack Query plugin](https://heyapi.dev/docs/openapi/typescript/plugins/tanstack-query)
- [Orval](https://orval.dev/)
- [Which OpenAPI codegen should you choose?](https://dev.to/nyaomaru/which-openapi-codegen-should-you-choose-openapi-typescript-vs-hey-api-vs-orval-vs-kubb-100p)
