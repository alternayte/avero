package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// EnvFile is the file that a person writes beside the application. `avero dev`
// reads it, so the loop needs no export in the shell.
const EnvFile = ".env"

// readEnvFile returns the variables of the .env file of the directory, in the
// order of the file. An absent file gives none.
//
// The parser reads KEY=VALUE. It skips a blank line and a line that starts
// with a number sign. It removes one pair of quotation marks around a value.
func readEnvFile(dir string) []string {
	file, err := os.Open(filepath.Join(dir, EnvFile))
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var out []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		out = append(out, name+"="+value)
	}
	return out
}

// hasEnv reports a variable that the list holds.
func hasEnv(list []string, name string) bool {
	for _, pair := range list {
		if strings.HasPrefix(pair, name+"=") && pair != name+"=" {
			return true
		}
	}
	return false
}
