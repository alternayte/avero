# The command that adds a component library

`avero new` writes an application with a stylesheet and no component opinion.
`avero ui add` puts a component library into an application of the ssr shape.

## The command

```
avero ui add basecoat [--style <name>]
```

`basecoat` is the only library today. An unknown library name fails with the
list of the known ones. The command runs against the root of an application,
in the shape that `avero new` wrote.

The command reads `avero.json` first. A shape that is not `ssr` fails with a
repair sentence, because a page of the spa shape and the api shape answers
JSON and renders no markup.

## The steps

1. Fetch the released tarball of Basecoat, and write its `dist/` tree under
   `assets/vendor/basecoat/`. `assets.PinPackage` carries this step. See [The
   package of a library](assets.md#the-package-of-a-library).
2. Add the import of the chosen style to `assets/css/app.css`, after the
   Tailwind import, when the line is absent.
3. Add the import of the script bundle to `assets/js/app.js`, when the line is
   absent.
4. Write `internal/ui/toaster.templ`, when the file is absent.
5. Patch `internal/ui/layout.templ`, under the rule that the next section
   states.
6. Print what changed, and print the two lines to paste by hand when step 5
   changed nothing.

Every step is idempotent. A second run of the command fetches nothing and
writes nothing.

## The eight styles

Basecoat publishes eight styles. `--style` names one:

```
vega  nova  maia  lyra  mira  luma  sera  rhea
```

`vega` is the default. A name that is not one of the eight fails with the
list.

## The rule of the patch

The command reads `internal/ui/layout.templ`, and compares the whole file
against the layout that the scaffold writes for this application. Two
outcomes follow.

The file that matches the scaffold, letter for letter, receives one change.
The command moves `@Toasts()` out of `<main>` and puts it at the end of
`<body>`, because the toaster is a fixed element of the page and not of the
content. It also removes the `Toasts` block that the scaffold layout defines,
because `internal/ui/toaster.templ` defines that name now.

The file that a person changed, in any way, receives no change. The command
prints the two lines to move by hand:

```
internal/ui/layout.templ does not match the scaffold, so it did not change.
Make two changes by hand:

  1. Move @Toasts() out of <main> and put it before </body>.
  2. Delete the templ Toasts() block, because internal/ui/toaster.templ
     defines that name now.
```

A second run of the command changes nothing. The file that already carries the
patch does not match the scaffold layout, so the command prints nothing and
leaves it alone.

## The vendor tree

The command writes the whole `dist/` tree of the tarball, stylesheets and
scripts together, under `assets/vendor/basecoat/`. `avero.lock` records the
address, the SHA-256 of the tarball and the sorted file list, so a build reads
the tree from disk and needs no network. See [The package of a
library](assets.md#the-package-of-a-library).

## The toaster

The command writes `internal/ui/toaster.templ` one time. The file belongs to
the application from that point, in the same way that every file of `avero
new` belongs to the application. A later version of Basecoat can change the
markup that its documentation states for a toast, and the command does not
reach back into an application to update the file that it already wrote. A
person who upgrades `avero ui add basecoat` to a later version therefore owns
the repair of `internal/ui/toaster.templ`, by hand, against the markup that
the new version states.

The server renders each toast, and Basecoat watches the document with a
`MutationObserver`. A toast that a Datastar patch inserts into the page
therefore gets its position, its timer and its pause on hover, with no script
of the application. Every toast carries a dismiss button, because the click
handler of the toaster closes a toast for a button of a footer only, and a
toast with no button never hides while the pointer rests on it. See [The
toaster](views.md#the-toaster).
