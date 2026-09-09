# The verification gate. See SDD section 7.
#
# .github/workflows/verify.yml runs the same steps. A change to a step belongs
# in both files.
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

# 5, 6. Integration tests. The asset suite needs the network. The CLI suite
# scaffolds an application and drives its commands. The broker suite arrives
# with S7.
integration:
    go test ./assets/... -race -tags=integration -timeout 10m
    go test ./internal/cli/... -tags=integration -run 'TestTheDoctor|TestTheRoutesAndThe|TestVerifyRuns' -timeout 20m
    go test ./cmd/avero/dev/... -tags=integration -run TestTheProductionBuild -timeout 10m
    go test ./mcp/... -tags=integration -timeout 20m
    @echo "integration: no broker test exists yet. S7 adds the first one."

# 7. Every generated file is current.
#
# The first step is the exact test: it regenerates in memory and compares. The
# second step is the one that the SDD states, scoped to the generated files so
# that uncommitted hand-written work does not trip it.
generate-check:
    go run ./internal/cmd/averogen -check .
    go generate ./...
    git diff --exit-code -- '*zz_generated*.go'

# 8, 9. Scaffold, build and test the three reference applications.
reference-apps: examples-check
    go test ./internal/cli/... -tags=integration -run TestEachShapeScaffolds -timeout 20m
    cd examples/blog && go build ./... && go test ./... -count=1
    cd examples/board && go build ./... && go test ./... -count=1
    cd examples/orders && go build ./... && go test ./... -count=1

# Write the three reference applications again. Run it after a change to a
# template, and commit the result.
examples:
    go run ./internal/cmd/averoexamples

# The examples match the templates of the scaffolder.
examples-check:
    go run ./internal/cmd/averoexamples -check

# 10. Measure DX-1 to DX-5.
budgets:
    go test ./internal/cli/... -tags=integration -run TestDX1 -timeout 15m -v
    go test ./cmd/avero/dev/... -tags=integration -run 'TestDX2|TestDX3' -timeout 15m -v
    @cat artifacts/verification.md
