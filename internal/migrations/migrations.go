// Package migrations reads the migration files of an application and the rows
// that drel already applied. The CLI prints them, and the boot check of the
// root package proves that none is pending. See the SDD, S14 and DX-8.
package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alternayte/drel"
)

// TrackingTable is the table that drel writes when it applies a migration.
const TrackingTable = "drel_migrations"

// Migration is one file pair of the migrations directory.
type Migration struct {
	// Version is the fourteen digit version of the file name.
	Version string
	// Name is the part of the file name after the version.
	Name string
}

// Read returns the migrations of the directory, in version order. An absent
// directory holds none.
func Read(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the migrations directory %s does not read", dir)
	}
	var out []Migration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version, rest, found := strings.Cut(strings.TrimSuffix(name, ".up.sql"), "_")
		if !found {
			continue
		}
		out = append(out, Migration{Version: version, Name: rest})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Applied returns the time of each applied migration, keyed by version. A
// database with no tracking table holds none.
func Applied(ctx context.Context, e *drel.Engine) map[string]string {
	out := map[string]string{}
	rows, err := e.Query(ctx, "SELECT version, applied_at FROM "+TrackingTable+" ORDER BY version")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		var at any
		if err := rows.Scan(&version, &at); err != nil {
			return out
		}
		out[version] = fmt.Sprint(at)
	}
	return out
}

// Pending returns the migrations that the database does not hold.
func Pending(ctx context.Context, e *drel.Engine, dir string) ([]Migration, error) {
	files, err := Read(dir)
	if err != nil {
		return nil, err
	}
	done := Applied(ctx, e)
	var out []Migration
	for _, m := range files {
		if _, ok := done[m.Version]; !ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// File returns the path of the up file or the down file of a migration.
func File(dir string, m Migration, direction string) string {
	return filepath.Join(dir, m.Version+"_"+m.Name+"."+direction+".sql")
}
