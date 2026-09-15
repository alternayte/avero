package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// value returns the effective value of one name, which is the last one that
// the list holds. os/exec reads the same one.
func value(list []string, name string) (string, bool) {
	out, held := "", false
	for _, pair := range list {
		if after, found := strings.CutPrefix(pair, name+"="); found {
			out, held = after, true
		}
	}
	return out, held
}

// writeEnv writes one .env file and returns its directory.
func writeEnv(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, EnvFile), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	return dir
}

// The parser reads the forms that a person writes in .env.
func TestTheEnvFileReadsEachForm(t *testing.T) {
	dir := writeEnv(t, strings.Join([]string{
		"# a comment",
		"",
		"PLAIN=one",
		"export EXPORTED=two",
		`DOUBLE="three"`,
		`SINGLE='four'`,
		"SPACED =  five  ",
		"EMPTY=",
		`ESCAPED="a\nb"`,
		`LITERAL='a\nb'`,
		`QUOTE="a\"b"`,
	}, "\n"))
	list, err := readEnvFile(dir)
	if err != nil {
		t.Fatalf("readEnvFile returned %v", err)
	}
	for name, want := range map[string]string{
		"PLAIN":    "one",
		"EXPORTED": "two",
		"DOUBLE":   "three",
		"SINGLE":   "four",
		"SPACED":   "five",
		"EMPTY":    "",
		"ESCAPED":  "a\nb",
		"LITERAL":  `a\nb`,
		"QUOTE":    `a"b`,
	} {
		got, held := value(list, name)
		if !held || got != want {
			t.Fatalf("%s is %q, want %q", name, got, want)
		}
	}
}

// A value between quotation marks can hold line breaks, so a key of PEM
// stands in the file as a person pastes it.
func TestTheEnvFileReadsAValueOfSeveralLines(t *testing.T) {
	key := "-----BEGIN RSA PRIVATE KEY-----\nMIIEow==\n-----END RSA PRIVATE KEY-----"
	dir := writeEnv(t, "BEFORE=one\nGITHUB_APP_PRIVATE_KEY=\""+key+"\"\nAFTER=two\n")
	list, err := readEnvFile(dir)
	if err != nil {
		t.Fatalf("readEnvFile returned %v", err)
	}
	got, held := value(list, "GITHUB_APP_PRIVATE_KEY")
	if !held || got != key {
		t.Fatalf("the key is %q", got)
	}
	// The lines of the value are not variables of their own, and the file
	// continues after the value.
	if _, held := value(list, "MIIEow=="); held {
		t.Fatal("a line of the value became a variable")
	}
	if after, _ := value(list, "AFTER"); after != "two" {
		t.Fatalf("the file stops after the value: AFTER is %q", after)
	}
}

// A value that opens a quotation mark that no line closes states the repair
// and names the line. See DX-7.
func TestAnUnterminatedValueIsAFault(t *testing.T) {
	dir := writeEnv(t, "A=one\nB=\"two\nC=three\n")
	_, err := readEnvFile(dir)
	if err == nil {
		t.Fatal("readEnvFile accepted an unterminated value")
	}
	for _, want := range []string{EnvFile, ":2:", "B", "→"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the fault holds no %q: %v", want, err)
		}
	}
}

// A variable of the shell wins over the file, because a person who exports a
// value states it for this one run. `just` with dotenv-load reads the same
// order.
func TestTheShellWinsOverTheEnvFile(t *testing.T) {
	dir := writeEnv(t, "KEY=from_the_file\nEMPTY_IN_FILE=\nONLY_IN_FILE=file\n")
	t.Setenv("KEY", "from_the_shell")
	t.Setenv("EMPTY_IN_FILE", "from_the_shell")

	list, err := Environment(dir)
	if err != nil {
		t.Fatalf("Environment returned %v", err)
	}
	for name, want := range map[string]string{
		"KEY":           "from_the_shell",
		"EMPTY_IN_FILE": "from_the_shell",
		"ONLY_IN_FILE":  "file",
	} {
		if got, _ := value(list, name); got != want {
			t.Fatalf("%s is %q, want %q", name, got, want)
		}
	}
	if !hasEnv(list, "EMPTY_IN_FILE") {
		t.Fatal("hasEnv reads the empty value of the file and not the value of the shell")
	}
}

// A variable that the file states and the shell clears holds no value, which
// is what the application reads.
func TestAnEmptyValueOfTheShellClearsTheFile(t *testing.T) {
	dir := writeEnv(t, "AVERO_SECRET=from_the_file\n")
	t.Setenv("AVERO_SECRET", "")
	list, err := Environment(dir)
	if err != nil {
		t.Fatalf("Environment returned %v", err)
	}
	if hasEnv(list, "AVERO_SECRET") {
		t.Fatal("hasEnv states a value that the shell cleared")
	}
}

// A directory with no .env gives the environment of the process.
func TestAnAbsentEnvFileGivesTheShell(t *testing.T) {
	list, err := Environment(t.TempDir())
	if err != nil {
		t.Fatalf("Environment returned %v", err)
	}
	if len(list) != len(os.Environ()) {
		t.Fatalf("the environment holds %d entries, want %d", len(list), len(os.Environ()))
	}
}
