package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvFile is the file that a person writes beside the application. `avero dev`
// and `avero migrate` read it, so a command needs no export in the shell.
const EnvFile = ".env"

// Environment returns the environment of a command that the CLI runs in the
// directory of an application: the variables of .env, and the variables of
// this process after them.
//
// The variable of the shell wins, because a person who exports a value states
// it for this one run. The file states the value that the application keeps.
// os/exec reads the last value of a name, so the order of the list is the
// order of the rule.
//
//	GITHUB_APP_PRIVATE_KEY="$(cat key.pem)" avero dev
func Environment(dir string) ([]string, error) {
	file, err := readEnvFile(dir)
	if err != nil {
		return nil, err
	}
	return append(file, os.Environ()...), nil
}

// readEnvFile returns the variables of the .env file of the directory, in the
// order of the file. An absent file gives none.
//
// The parser reads KEY=VALUE. It skips a blank line and a line that starts
// with a number sign. It reads `export KEY=VALUE` as KEY=VALUE.
//
// A value between quotation marks can hold line breaks, so a key of PEM
// stands in the file as a person pastes it. A value between double quotation
// marks carries the escapes \n, \r, \t, \\ and \". A value between single
// quotation marks carries the characters that it holds and no escape.
func readEnvFile(dir string) ([]string, error) {
	name := filepath.Join(dir, EnvFile)
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, nil
	}
	out, err := parseEnv(string(body))
	if err != nil {
		return nil, fmt.Errorf("%s%w%s", name, err,
			hint("Close the quotation mark of the value. Run the command again."))
	}
	return out, nil
}

// parseEnv reads the variables of one env file.
func parseEnv(body string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, raw, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		raw = strings.TrimLeft(strings.TrimSpace(raw), " \t")
		if quote := openingQuote(raw); quote != 0 {
			value, last, ok := quoted(lines, i, quote)
			if !ok {
				return nil, fmt.Errorf(":%d: the value of %s opens a quotation mark that no line closes", i+1, name)
			}
			out = append(out, name+"="+value)
			i = last
			continue
		}
		out = append(out, name+"="+raw)
	}
	return out, nil
}

// openingQuote returns the quotation mark that a value opens, or zero.
func openingQuote(raw string) byte {
	if raw == "" {
		return 0
	}
	if raw[0] == '"' || raw[0] == '\'' {
		return raw[0]
	}
	return 0
}

// quoted reads a value between quotation marks, which can span lines. It
// returns the value, the index of the last line that it read, and whether a
// line closed the value.
func quoted(lines []string, first int, quote byte) (string, int, bool) {
	// The value starts after the name, the equals sign and the quotation
	// mark.
	line := strings.TrimSpace(lines[first])
	line = strings.TrimPrefix(line, "export ")
	_, raw, _ := strings.Cut(line, "=")
	rest := strings.TrimLeft(strings.TrimSpace(raw), " \t")[1:]

	var b strings.Builder
	for i := first; i < len(lines); i++ {
		if i > first {
			rest = lines[i]
			b.WriteByte('\n')
		}
		end, ok := closingQuote(rest, quote)
		if !ok {
			b.WriteString(rest)
			continue
		}
		b.WriteString(rest[:end])
		return unescape(b.String(), quote), i, true
	}
	return "", first, false
}

// closingQuote returns the position of the quotation mark that ends a value.
// A double quotation mark that a backslash marks stands inside the value.
func closingQuote(line string, quote byte) (int, bool) {
	for i := 0; i < len(line); i++ {
		if quote == '"' && line[i] == '\\' {
			i++
			continue
		}
		if line[i] == quote {
			return i, true
		}
	}
	return 0, false
}

// unescape reads the escapes of a value between double quotation marks. A
// value between single quotation marks holds the characters that it states.
func unescape(value string, quote byte) string {
	if quote != '"' || !strings.Contains(value, "\\") {
		return value
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '\\', '"':
			b.WriteByte(value[i])
		default:
			b.WriteByte('\\')
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

// hasEnv reports a variable of the list that holds a value.
//
// os/exec reads the last value of a name, so the check reads the last one as
// well. A variable that .env states and the shell clears therefore holds no
// value here, as it holds none in the application.
func hasEnv(list []string, name string) bool {
	for i := len(list) - 1; i >= 0; i-- {
		pair := list[i]
		if !strings.HasPrefix(pair, name+"=") {
			continue
		}
		return pair != name+"="
	}
	return false
}
