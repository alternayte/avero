# The verification gate. See SDD section 7.
#
# Steps 5, 6, 8, 9 and 10 need subsystems that do not exist yet. Each one
# prints the subsystem that must land before the step can run. Add the real
# command when that subsystem lands. Do not delete a step.

default: verify

verify: fmt vet lint test generate-check integration reference-apps budgets

# 1. gofmt reports no file.
fmt:
    @out=$(gofmt -l .); if [ -n "$out" ]; then echo "gofmt: these files need a format:"; echo "$out"; exit 1; fi

# 2. go vet passes.
vet:
    go vet ./...

# 3. golangci-lint passes.
lint:
    golangci-lint run

# 4. Unit tests pass with the race detector.
test:
    go test ./... -race -count=1

# 5, 6. Integration tests against PostgreSQL and RabbitMQ.
integration:
    @echo "integration: no integration test exists yet. S7 adds the first one."

# 7. Every generated file is current.
#
# The first step is the exact test: it regenerates in memory and compares. The
# second step is the one that the SDD states, scoped to the generated files so
# that uncommitted hand-written work does not trip it.
generate-check:
    go run ./internal/cmd/averogen -check .
    go generate ./...
    git diff --exit-code -- '*zz_generated.go'

# 8, 9. Scaffold, build and test the three reference applications.
reference-apps:
    @echo "reference-apps: no scaffolder exists yet. S14 adds it."

# 10. Measure DX-1 to DX-5.
budgets:
    @echo "budgets: no scaffolded application exists yet. S14 adds it."
