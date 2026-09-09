# Board

Board is an Avero application of the spa shape.

## Run it

```
cp .env.example .env
avero migrate up
go run .
```

The application answers on http://localhost:8080.

## The commands

```
avero doctor          prove the configuration, the database and the broker
avero routes          print the routes
avero modules         print the contribution of each module
avero schema          print the models
avero generate        write the generated files
avero build           generate, build the assets and compile the binary
avero migrate status  print the migrations
avero verify          run the gate
```

## The layout

```
main.go                     the composition root
config.go                   every variable that the application reads
wire.go                     the router, the middleware and the modules
internal/features/          one directory for each feature
internal/ui/                the views. It imports no feature package.
migrations/                 the SQL files
assets/                     the source of the stylesheet and of the script
```

Read `AGENTS.md` before you change the code.
