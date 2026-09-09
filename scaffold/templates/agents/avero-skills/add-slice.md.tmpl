# Add a feature slice

A slice holds its handlers, its models, its views and its tests in one
directory. A slice imports no other slice.

## Steps

1. Write the directory `internal/features/<name>/`.
2. Write the test first. Copy `internal/features/posts/posts_test.go` and drive
   the routes that you plan.
3. Write `module.go` with the Module type. It states `Name() string` and it
   implements the optional interfaces that the feature needs:
   `Routes(r *avero.Router)`, `Migrations() fs.FS` and `Describe()`.
4. Write the input types in `input.go`. Give each field its source tag and its
   `validate` tag. Run `avero generate` to write the binding and the
   validation.
5. Write the handlers in `handlers.go`. A handler has the shape
   `func (m *Module) Name(c *avero.Ctx, in Input) (avero.Response, error)`.
6. Write the views in `internal/ui/`. The view reads the old input and the
   field errors with `view.Old` and `view.Error`.
7. Register the module in `wire.go`.
8. Write the migration with `avero migrate new <name>`.

## The command that proves the work

```
avero verify
```
