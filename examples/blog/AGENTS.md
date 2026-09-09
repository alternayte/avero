# AGENTS.md

Blog is an Avero application. Avero owns the lifecycle, the
configuration, the telemetry and the routing. This application owns the
semantics.

## The gate

```
avero verify          the gate. Run it before you say that the work is done.
```

`avero verify --json` writes one record for each step, with the name, the
status, the time and, for a failure, the file, the line and the message. The
order never changes.

## The other commands

```
avero generate        write the generated files after a change to an input type
avero slice <name>    write a new feature slice
avero doctor          prove the configuration, the database and the broker
avero routes          print the routes
avero routes --openapi  write the OpenAPI description of the API
avero modules         print the contribution of each module
avero schema          print the models
avero dev             run the application and rebuild it on each change
go test ./... -race
```

## Design rules

1. No global mutable state. No facade. No package level singleton.
2. No runtime reflection on a request path. Generate the code instead.
3. Dependencies are struct fields. The constructor takes them. `main.go`
   supplies them.
4. Generated code is plain Go. A person can read it, edit it and delete it.
5. Avero owns the lifecycle. This application owns the semantics.
6. Explicit beats short. Do not add magic to save a line.
7. One feature directory holds its handlers, its models, its views and its
   tests. A feature does not import another feature.
8. Move a fault to compile time when you can.

## The layout

```
main.go                     the composition root
config.go                   every variable that the application reads
wire.go                     the router, the middleware and the modules
internal/features/          one directory for each feature. Each holds a
                            model/ package that drel reads, and a migrations/
                            package that drel writes.
internal/ui/                the views. They import no feature package.
                            A page is a .templ file. Run `avero generate`
                            after a change.
drel.yaml                   the model packages and the migrations of drel
assets/                     the source of the stylesheet and of the script
avero.json                  the shape and the asset pipeline
```

## Error rules

- An error names a path, a line and a column in a file that a person wrote.
- An error carries one sentence that states the repair.
- A fault appears before the process starts. `avero doctor` runs at boot.

## Test rules

- Write the test before the code that it judges.
- An acceptance test drives real HTTP against a real database. Do not use a
  mock at that level.
- Never relax a test to make the gate pass. Never skip a test.
- Never edit a generated file by hand. Change the input type or the model, then
  run `avero generate`.
- A store writes no SQL string. It reads and writes with the typed API of drel,
  so a name fault appears at compile time.

## Writing standard

Write every document, comment and commit message in ASD-STE100 Simplified
Technical English. Use the active voice. Use one word for one meaning. Do not
use contractions.

## The skills

`.claude/skills/` holds one skill for each task that repeats:

```
.claude/skills/add-slice/SKILL.md            add a feature
.claude/skills/add-client/SKILL.md           call another service
.claude/skills/add-projection/SKILL.md       add a read model
.claude/skills/add-inbox-handler/SKILL.md    consume a message
```

Each skill follows the Agent Skills format: a name and a description in the
frontmatter, then the steps, the files to write and the command that proves
the work. A coding agent reads them without a setting.
