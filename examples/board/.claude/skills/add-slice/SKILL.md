---
name: add-slice
description: Use when the task adds a feature to this Avero application, such as a new resource, a new set of routes or a new table. It writes the module, the input types, the handlers, the store, the test and the migration, and it registers the module.
---

# Add a feature slice

One slice holds the routes, the handlers, the input types, the store and the
tests of one feature. A slice imports no other slice.

## Steps

1. Write the slice with the command:

   ```
   avero slice <name>
   ```

   The command writes `internal/features/<name>s/`, the migration, and the
   test. It registers the module in `wire.go`.

2. Read `internal/features/<name>s/module.go`. Change the routes to the ones
   that the task states.

3. Change the input types in `input.go`. Give each field its source tag
   (`path`, `query`, `form` or `json`) and its `validate` tag. Run
   `avero generate` after each change.

4. Change the handlers in `handlers.go`. A handler has the shape
   `func (m *Module) Name(c *avero.Ctx, in Input) (avero.Response, error)`.
   It returns a Response. It never writes to the ResponseWriter.

5. Change the store in `store.go`. A read and a write run inside the
   transaction of the request when one is open.

6. Change the table in `migrations/`. Apply it with `avero migrate up`.

7. For a page, write the view in `internal/ui/`. The view reads the old input
   with `view.Old` and the field error with `view.Error`. `internal/ui`
   imports no feature package.

## Rules

- Write the test before the code that it judges.
- A feature does not import another feature.
- Never edit a file that starts with `zz_generated`. Change the input type and
  run `avero generate`.

## The command that proves the work

```
avero verify
```
