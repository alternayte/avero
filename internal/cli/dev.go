package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/cmd/avero/dev"
	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
)

// runDev runs the development loop.
func runDev(ctx context.Context, s Streams, args []string) int {
	port := 8080
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--port" || a == "-port":
			i++
			if i >= len(args) {
				return failf(s, "avero dev: --port names no port\n  → Write --port 8080")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return failf(s, "avero dev: the port %q is not a number\n  → Write --port 8080", args[i])
			}
			port = n
		case strings.HasPrefix(a, "--port="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--port="))
			if err != nil {
				return failf(s, "avero dev: the port %q is not a number\n  → Write --port 8080", a)
			}
			port = n
		default:
			return failf(s, "avero dev: the flag %q is not known\n  → Run `avero dev` or `avero dev --port 8080`", a)
		}
	}

	dir := dirOf(s)
	project, err := LoadProject(dir)
	if err != nil {
		return fail(s, err)
	}
	if env := os.Getenv("AVERO_ENV"); env != "" && env != dev.EnvDevelopment {
		return failf(s, "avero dev: AVERO_ENV is %q\n  → Set AVERO_ENV=development, because the loop rebuilds and restarts the application", env)
	}

	// The loop reads .env, so a person runs one command and no export.
	environment := append(os.Environ(), readEnvFile(dir)...)
	if !hasEnv(environment, "AVERO_SECRET") {
		return failf(s, "avero dev: AVERO_SECRET is absent\n  → Copy .env.example to .env, which already holds a key for this application")
	}

	server, err := dev.New(dev.Config{
		Dir:      dir,
		Port:     port,
		Env:      dev.EnvDevelopment,
		Out:      s.Out,
		AssetDir: filepath.Join(dir, filepath.FromSlash(assets.OutDir)),
		BuildCSS: func(ctx context.Context) (string, error) {
			m, err := assets.Build(ctx, project.assetConfig(dir, false))
			if err != nil {
				return "", err
			}
			return m.Asset("app.css"), nil
		},
		BuildGo: func(ctx context.Context) error { return buildBinary(ctx, dir, project) },
		Start: func(ctx context.Context, port int) (dev.Process, error) {
			return startBinary(ctx, dir, project, port, environment, s)
		},
	})
	if err != nil {
		return fail(s, err)
	}
	return fail(s, server.Run(ctx))
}

// buildBinary writes the generated files and compiles the application.
func buildBinary(ctx context.Context, dir string, project *Project) error {
	generated := time.Now()
	if _, err := codegen.Generate(dir); err != nil {
		return err
	}
	if _, err := client.Generate(dir); err != nil {
		return err
	}
	_ = generated
	// The loop builds without the version control stamp, because the stamp
	// reads the repository on each build and the loop builds often.
	out, err := goCommand(ctx, dir, "build", "-buildvcs=false", "-o", devBinary(project), ".")
	if err != nil {
		return fmt.Errorf("avero dev: the compiler failed\n%s\n  → Repair the fault that the compiler names. The loop builds again after the next change",
			strings.TrimSpace(out))
	}
	return nil
}

// devBinary returns the path of the binary that the loop runs.
func devBinary(project *Project) string {
	return filepath.Join(".avero", "bin", project.Name+"-dev")
}

// startBinary starts the application on one port.
func startBinary(ctx context.Context, dir string, project *Project, port int,
	environment []string, s Streams,
) (dev.Process, error) {
	cmd := exec.CommandContext(ctx, "./"+devBinary(project))
	cmd.Dir = dir
	cmd.Env = append(append([]string(nil), environment...),
		"PORT="+strconv.Itoa(port),
		"AVERO_ENV="+dev.EnvDevelopment)
	cmd.Stdout = s.Out
	cmd.Stderr = s.Err
	// The application ends with its own group, so no child of it stays
	// behind and holds the output of the loop.
	dev.Group(cmd)
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("avero dev: the application does not start: %w\n  → Run `avero build` and repair the fault that it names", err)
	}
	return newProcess(cmd), nil
}

// process is one running application. It reports its own end, so the loop
// states the fault at once instead of waiting for the health endpoint.
type process struct {
	cmd    *exec.Cmd
	exited chan error
	once   sync.Once
}

// newProcess watches one command.
func newProcess(cmd *exec.Cmd) *process {
	p := &process{cmd: cmd, exited: make(chan error, 1)}
	go func() {
		err := cmd.Wait()
		p.exited <- fmt.Errorf("avero dev: the application ended: %w\n  → Read the output above, then repair the fault that it names", err)
	}()
	return p
}

// Exited returns a channel that carries the end of the application.
func (p *process) Exited() <-chan error { return p.exited }

// Stop ends the application.
func (p *process) Stop() error {
	if p.cmd.Process == nil {
		return nil
	}
	p.once.Do(func() { dev.KillGroup(p.cmd) })
	select {
	case <-p.exited:
	case <-time.After(5 * time.Second):
	}
	return nil
}
