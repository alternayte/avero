# Configuration

An application reads its configuration one time, at start. A variable that is
absent stops the process before it serves.

## The type

```go
type Config struct {
	avero.BaseConfig

	// DatabaseURL is the address of the database.
	DatabaseURL string `env:"DATABASE_URL" default:"file:blog.db"`
}
```

`avero.Load[Config](ctx)` fills the type from the environment. The loader
reports every fault of one load, not only the first.

## The tags

| Tag | Meaning |
|---|---|
| `env:"NAME"` | the name of the environment variable |
| `env:"NAME,required"` | the process stops when the variable is absent or empty |
| `env:"NAME,secret"` | no log line and no error message prints the value |
| `default:"value"` | the value that applies when the variable is absent |

An embedded struct takes no prefix, so `PORT` stays `PORT`. A named struct
takes the name of its tag as the prefix, so `OTel OTelConfig \`env:"OTEL"\``
reads `OTEL_SERVICE_NAME`.

## The variables of BaseConfig

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | the port of the HTTP server |
| `LOG_LEVEL` | `info` | debug, info, warn or error |
| `LOG_FORMAT` | `json` | json or text |
| `SHUTDOWN_GRACE` | `15s` | the deadline that Stop receives |
| `MIGRATE_ON_BOOT` | `false` | apply the pending migrations at start |
| `AVERO_ENV` | `development` | development or production |
| `AVERO_SECRET` | required | the key that signs the CSRF cookie and the flash cookie |
| `OTEL_*` | — | the standard OpenTelemetry variables |

Write a new key with `openssl rand -hex 32`.

## The boot checks

A check proves one condition before the first component starts.

```go
app := avero.New(cfg.BaseConfig,
	avero.WithHandler(handler),
	avero.WithChecks(
		avero.SecretCheck(cfg.Secret),
		avero.DatabaseCheckOn(engine),
		avero.MigrationCheckOn(engine, "migrations"),
	))
```

`avero doctor` runs the same list and prints one row for each check. A failed
row states the fault and the repair. `avero doctor` also loads the
configuration and reads the database address through the `DSN` function of
the service. It reports the address that the application uses.

A configuration with a fault still yields the values that did load, so the
doctor can state a composed address that is incomplete. The table above names
the absent variable in the same report.

## The .env file

`avero dev` and `avero migrate` read `.env` beside the application, so a
command needs no export in the shell.

A variable that the shell holds wins over the file. A person who exports a
value states it for this one run:

```
GITHUB_APP_PRIVATE_KEY="$(cat key.pem)" avero dev
```

A value between quotation marks can hold line breaks, so a key of PEM stands
in the file as a person pastes it:

```
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----
MIIEow...
-----END RSA PRIVATE KEY-----"
```

A value between double quotation marks carries the escapes `\n`, `\r`, `\t`,
`\\` and `\"`. A value between single quotation marks carries the characters
that it holds and no escape. A value that opens a quotation mark that no line
closes stops the command, and the fault names the line.

The application itself reads the environment of its process and no file, so a
deployment states its variables as a container or a unit file does.

## A secret

`avero.Secret` prints as `********` in a log line, in an error message and in a
JSON document. Call `Reveal` to read the value.
