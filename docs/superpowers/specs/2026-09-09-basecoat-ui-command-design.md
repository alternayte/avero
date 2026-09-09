# Design — `avero ui add basecoat`

Date: 2026-09-09. Written in ASD-STE100.

## 1. Purpose

Avero writes an application with 25 lines of stylesheet and no component
opinion. A person who wants a button, a dialog or a toast writes the styles
alone. Basecoat is a component library of Tailwind CSS classes, and it needs
no React and no Node.js.

This design adds one command that puts Basecoat into an Avero application. The
command works on a new application and on an application that a person already
changed.

## 2. Scope

In scope:

- A pin path for a stylesheet, beside the pin path for a module.
- The command `avero ui add basecoat [--style <name>]`.
- A toaster component that renders the toasts of Avero in the Basecoat markup.
- The ssr example, which uses the command and proves it in the gate.

Out of scope:

- A templ wrapper for each Basecoat component. A person copies the markup of
  the Basecoat documentation. A later design can add the wrappers.
- The spa shape and the api shape. Both answer JSON.
- The Basecoat chart helper. It needs a second script.

## 3. Decisions

**D1. Avero fetches the files. Avero carries no copy.** The command reads
jsDelivr and writes the two files into the application. Avero holds no release
of another project, so a person picks the Basecoat version and Avero needs no
release for it. This follows design rule 5.

**D2. The command patches a layout that it recognizes, and prints for one that
it does not.** A person must be able to add Basecoat to an application that
runs. A command that overwrites `internal/ui/layout.templ` destroys work. A
command that never patches makes a new application slower to start. The command
therefore compares the layout with the scaffold, and it patches only an exact
match. In every other case it changes nothing and prints the lines to paste.

**D3. The server renders a toast. The client fires none.** Section 5 states the
measurement that supports this.

**D4. The default style is `vega`, and `--style` names another.** Basecoat
ships eight. The lock records the one that the application uses.

## 4. The pin of a stylesheet

`assets/lock.go` holds `PinJS`, which fetches a bundled ES module, rewrites the
imports of the CDN, writes `assets/js/vendor/<name>.js`, and records the URL and
the two hashes in `avero.lock`.

A stylesheet needs the same work without the rewrite. Add:

```go
func PinCSS(ctx context.Context, cfg PinConfig, name, url string) error
```

It writes `assets/css/vendor/<name>.css` and records the pin under a third map
of the lock, `css`. `Lock` gains `CSS(name)` and `SetCSS(name, pin)`, and
`lockJSON` gains a `css` member. An older lock that holds no `css` member reads
without a fault.

`PinCSS` states no address of its own. The caller names the URL, because a
stylesheet has no package resolution of the kind that `Resolve` performs for a
module.

## 5. The toaster

### 5.1 The measurement

The Basecoat source states four facts. `dist/js/toast.js` and
`dist/js/basecoat.js` of version 1.0.2 hold them.

1. `.toaster` is `position: fixed; bottom: 0; z-index: 50` with
   `flex-direction: column-reverse`. The position needs no JavaScript.
   `data-align` takes `start`, `center` or `end`.
2. Basecoat registers two selectors:
   `#toaster:not([data-toaster-initialized])` and
   `.toast:not([data-toast-initialized])`.
3. `basecoat.js` runs a `MutationObserver` on `document.body`. A node that
   arrives and matches a registered selector initializes itself. A patch of
   Datastar therefore needs no call of `initAll`.
4. `initToaster` also adopts the toasts that the server already wrote:
   `toaster.querySelectorAll('.toast:not([data-toast-initialized])')`.

A server-rendered toast therefore gets the position, the timer, the pause on
hover and the stack. The application loses one thing only: it cannot fire a
toast from the client with no request. The method stays available, because
`initToaster` attaches it, so `document.getElementById('toaster').toast({...})`
works for a person who wants it.

The htmx examples of Basecoat fire a toast from a header and a script, because
htmx swaps one target and a toast is out of band. Datastar patches by selector,
so the server patches `#toaster`. The server-rendered path is the shorter path
under Datastar, and not a compromise.

### 5.2 The dismiss

The click handler of the toaster closes a toast for one case only:

```js
const actionLink   = event.target.closest('.toast footer a');
const actionButton = event.target.closest('.toast footer button');
```

A click on the body of a toast does nothing. A toast under the pointer keeps
its timer paused, so a toast with no button never hides while the pointer stays
on it. Every toast therefore carries the footer that `createToast` writes for a
`cancel` value:

```html
<footer>
  <button type="button" class="btn h-6 text-xs px-2.5 rounded-sm"
          data-variant="outline" data-toast-cancel>Dismiss</button>
</footer>
```

The handler of the toaster closes the toast, so the application adds no script.

### 5.3 The markup

`internal/ui/basecoat.templ` holds `Toasts()`:

```html
<div id="toaster" class="toaster" data-align="end">
  <!-- for each toast -->
  <div class="toast" role="status|alert" aria-atomic="true"
       data-category="{level}">
    <div class="toast-content">
      {icon}
      <section><h2>{message}</h2></section>
      <footer>…the dismiss button…</footer>
    </div>
  </div>
</div>
```

`role` is `alert` for the error level and `status` for every other level.

The four levels of Avero are `info`, `success`, `warning` and `error`, and they
are the four categories of Basecoat. The component needs no mapping table.

The component renders the container even when it holds no toast, so a Datastar
patch replaces `#toaster` in outer mode and needs no second target.

The component carries the four icons of Basecoat as inline SVG, because
`createToast` writes them and the server writes no `createToast` call.

## 6. The command

```
avero ui add basecoat [--style vega]
```

`internal/cli/ui.go` holds `runUI`. `cli.go` registers the command with the
usage line `avero ui add <library> [--style <name>]`. `basecoat` is the only
library today, and an unknown name fails with the list of the known ones.

The steps, in order:

1. Read `avero.json`. A shape that is not `ssr` fails with a repair sentence.
2. `PinCSS` fetches
   `https://cdn.jsdelivr.net/npm/basecoat-css@<version>/dist/basecoat.<style>.cdn.min.css`
   into `assets/css/vendor/basecoat.css`.
3. `PinJS` fetches `.../dist/js/all.min.js` into
   `assets/js/vendor/basecoat.js`.
4. Add `@import "vendor/basecoat.css";` to `assets/css/app.css` after the
   Tailwind import, when the line is absent.
5. Add `import "basecoat";` to `assets/js/app.js`, when the line is absent.
6. Patch `internal/ui/layout.templ` under the rule of D2.
7. Write `internal/ui/basecoat.templ`, when the file is absent.
8. Print what changed, and print the lines to paste when step 6 patched
   nothing.

Every step is idempotent. A second run fetches nothing and writes nothing.

The version of Basecoat is a constant of the `assets` package, beside
`TailwindVersion`.

### 6.1 The patch of the layout

The command reads `internal/ui/layout.templ` and compares it with the layout
that the scaffold writes for this shape and this hypermedia library. An exact
match receives one change: `@Toasts()` moves out of `<main>` and to the end of
`<body>`. The toaster is a fixed element of the page and not of the content.

The layout needs no second link element. Tailwind compiles one stylesheet, and
step 4 imports the vendor file into it, so the layout keeps the one link that
it already holds.

A layout that does not match receives no change. The command prints the line to
move and the position that it belongs in.

## 7. The ssr example

The blog gets the Basecoat classes on its form, its buttons and its list of
posts. `internal/cmd/averoexamples` runs `avero ui add basecoat` for the blog,
so the gate proves the command on every run and the example never drifts.

The starter assets of the example already ignore the output directory, so a
clone holds no vendor file. `averoexamples -assets` fetches them.

## 8. The tests

| Test | Proves |
|---|---|
| `PinCSS` writes the vendor file and the lock | section 4 |
| `PinCSS` fetches nothing on a second run | the idempotence |
| `PinCSS` fails on a hash that does not match | the supply chain |
| An older lock with no `css` member reads | the compatibility |
| The command on a scaffolded application patches the layout | D2 |
| The command on a changed layout writes nothing and prints the lines | D2 |
| The command runs two times and gives one result | the idempotence |
| The command on the spa shape fails with a repair sentence | step 1 |
| `Toasts()` renders `#toaster` when no toast is present | 5.3 |
| `Toasts()` renders `data-category` and the `alert` role for an error | 5.3 |
| `Toasts()` renders the dismiss button | 5.2 |
| An acceptance test posts the form of the blog, follows the redirect, and reads one `.toast` with `data-category="success"` inside `#toaster` | the whole path |

The command fetches over HTTP, so each test passes a `Fetcher` of the test, as
`assets/pin_test.go` already does.

## 9. The documents

- `docs/assets.md` gains a section for the pin of a stylesheet.
- `docs/views.md` gains the toaster and the dismiss button.
- A new `docs/ui.md` states the command, the styles and the rule of the patch.
- `README.md` names the command in the list.
- `CHANGELOG.md` records the command.

## 10. The relation to the SDD

The SDD names no `avero ui` command. S11 owns the asset pipeline and S14 owns
the CLI, and this design adds one command to each. The SDD needs one line in
S11 for `PinCSS` and one line in S14 for the command. The change adds no
third-party Go dependency and no database table.

## 11. The risks

**R1. The Basecoat markup changes between versions.** The lock holds the hash,
so a change is visible. The toaster markup of `internal/ui/basecoat.templ`
belongs to the application after the command wrote it, so a person owns the
repair. State this in `docs/ui.md`.

**R2. `all.min.js` is not an ES module.** The bundle is an IIFE. esbuild takes
an IIFE as an entry of a bundle and gives the same bytes, and the import of
`app.js` runs it for its side effect. The first task of the plan must prove
this with a build, because a fault here changes step 5.

**R3. A person changes the layout later.** The command runs one time, so a
later change to the scaffold layout does not reach an application that already
holds Basecoat. This is the same rule that every scaffolded file follows.
