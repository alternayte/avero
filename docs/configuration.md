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
row states the fault and the repair.

## A secret

`avero.Secret` prints as `********` in a log line, in an error message and in a
JSON document. Call `Reveal` to read the value.
