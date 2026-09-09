# The agent surface

Avero makes an application legible for an agent that a person operates. It adds
no feature that calls a language model.

## The scaffolded files

`avero new` writes `AGENTS.md` with the eight design rules, the layout and the
gate. It writes four skills:

```
.avero/skills/add-slice.md
.avero/skills/add-projection.md
.avero/skills/add-inbox-handler.md
.avero/skills/add-client.md
```

Each skill states the steps, the files to write and the command that proves the
work.

## The MCP server

```
avero mcp
```

The command speaks the Model Context Protocol over stdio. Point your agent at
it with the directory of the application as the working directory.

| Tool | Result |
|---|---|
| `list_modules` | the contribution and the description of every module |
| `describe_module` | one module in full |
| `list_routes` | the method, the pattern, the handler and the middleware |
| `describe_model` | the module, the table and the fields |
| `scaffold_slice` | a new feature slice, written and registered |
| `run_verify` | the record set of the gate |
| `explain_error` | the position in your code, and the repair |

Every tool except `scaffold_slice` writes nothing. The server runs no command
that a caller names: it asks the application for its own inspection, and it
runs the fixed steps of the gate.

`mcp/schema.json` states the shape of each result.

## explain_error

A fault in a generated file maps back to the code that you wrote:

```
zz_generated.go:59:3: cannot use v (variable of type string) as int value in assignment to in.Title
```

answers with the field `Title` of `CreateInput` in `input.go`, and the repair
*Repair the field Title of CreateInput in input.go, then run `avero generate`*.

## The reports

Each report carries the address of its schema, and each order is stable, so two
runs give one answer.

| Command | Schema |
|---|---|
| `avero verify --json` | `verify/schema.json` |
| `avero routes --json` | `router/schema.json` |
| `avero modules --json` | `module/schema.json` |
| `avero schema --json` | `module/schema_models.json` |
| `avero doctor --json` | `host/doctor_schema.json` |
| `avero build` | `assets/schema.json` |

## The record of the gate

```json
{
  "name": "vet",
  "status": "fail",
  "duration_ms": 412,
  "message": "vet: ./broken.go:3:28: cannot use \"x\" as int value",
  "file": "broken.go",
  "line": 3,
  "column": 28
}
```
