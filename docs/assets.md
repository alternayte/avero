# Assets

The asset pipeline has three tiers. Tier 0 is the default and needs no Node.js.

## Tier 0

`avero build` bundles the entry points with esbuild as a Go library, and it
compiles the stylesheet with the Tailwind standalone binary. It resolves a
relative path and a vendored package only.

```
avero js pin nanostores https://cdn.jsdelivr.net/npm/nanostores@0.11.3/+esm
```

The command writes the module into `assets/js/vendor/` and records the address
and the SHA-256 in `avero.lock`. A second run changes nothing. A body that does
not match the hash stops the command.

Import the module by its bare name:

```js
import { atom } from "nanostores";
```

A CDN keeps a peer dependency outside its bundle and names it with an absolute
path of its own host. The pin rewrites such an import to the name of the
package, so the bundler resolves it in the vendor directory. `avero.lock`
records the hash of the file that the pin wrote and the hash of the bytes that
the address answered.

## React and TanStack Query

The spa shape carries React 19 and TanStack Query, and it needs no Node.js.
`avero new --shape spa` pins four modules:

```
react
react/jsx-runtime
react-dom/client
@tanstack/react-query
```

esbuild transforms the JSX with the automatic runtime, so a component file
needs no import of React, and it bundles the application and the vendored
modules into one file of about 230 kilobytes.

## Tier 1

```
avero assets init
npm install some-package
avero build
```

The command writes a `package.json`, and esbuild then resolves a bare specifier
against `node_modules`. The build command stays `avero build`.

## Tier 2

Write the bundler command in `avero.json`:

```json
{
  "assets": {
    "tier": "external",
    "command": ["npm", "run", "build"]
  }
}
```

The command must write `assets/dist/` and a `manifest.json` with the shape of
`assets/schema.json`.

## The manifest

`avero build` writes each file under its content hash, such as
`app.a1b2c3d4e5f6.css`, and records it in `assets/dist/manifest.json`. A changed
file therefore gets a new name, and the browser holds the old one no more.

```go
//go:embed all:assets/dist
var dist embed.FS

m, err := avero.LoadManifest(dist, "assets/dist/manifest.json")
r.Mount("/assets/", http.StripPrefix("/assets/", avero.AssetHandler(files, m)))
```

`ui.Asset("app.css")` answers the hashed path. The handler serves the files that
the manifest names and nothing else, with a cache of one year.

## The configuration

```json
{
  "assets": {
    "entries": ["js/app.js"],
    "tailwind": {
      "input": "assets/css/app.css",
      "asset": "css/app.css",
      "version": "4.1.11"
    }
  }
}
```

A project that names no Tailwind input runs no Tailwind. The binary lands in
`.avero/bin/`, pinned by version and by hash in `avero.lock`.

## The development loop

`avero dev` serves the assets from the disk, because the binary carries the
copy of its last build. A change to a CSS file rebuilds the stylesheet and swaps
the link element with no reload.
