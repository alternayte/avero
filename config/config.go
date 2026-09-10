// Package config loads a configuration struct from environment variables.
//
// Load reads every field, collects every fault, and returns a report that
// names the source of each value. It does not stop at the first fault. See the
// SDD, section 6, S1.
//
// The loader uses reflection one time at start. No request path calls it.
package config

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// Loader reads the values. Give it a Lookup to read from something other than
// the process environment. A nil Lookup reads os.LookupEnv.
type Loader struct {
	// Lookup returns the value of a variable and reports whether it is set.
	Lookup func(name string) (string, bool)
}

func (l Loader) lookup(name string) (string, bool) {
	if l.Lookup == nil {
		return os.LookupEnv(name)
	}
	return l.Lookup(name)
}

// Load reads the process environment into a new T. It returns a *FaultList
// when T holds a fault, and a nil value beside it. Call Exit to print the
// faults and to get the exit code.
func Load[T any](ctx context.Context) (*T, error) {
	cfg, _, err := LoadFrom[T](ctx, Loader{})
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadFrom reads l into a new T and returns a report of every field. It
// returns a *FaultList when T holds a fault, together with the value the
// loader already filled and the report of every field it read before the
// fault. A caller such as `avero doctor` reads the value that did load
// beside the fault that did not.
func LoadFrom[T any](ctx context.Context, l Loader) (*T, *Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	cfg := new(T)
	t := reflect.TypeOf(*cfg)
	if t == nil || t.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("config: %s is not a struct, so the loader cannot fill it", reflect.TypeOf(cfg).Elem())
	}
	w := &walker{loader: l, root: t.Name(), report: &Report{}, faults: &FaultList{}}
	if w.root == "" {
		w.root = "config"
	}
	w.walk(reflect.ValueOf(cfg).Elem(), "", "")
	if err := w.faults.err(); err != nil {
		// The report and the value go back with the faults. `avero doctor`
		// prints the whole table and proves the parts that did load, even
		// when a variable is absent. See DX-8.
		w.faults.Report = w.report
		return cfg, w.report, err
	}
	return cfg, w.report, nil
}

// walker fills one struct and collects the faults it finds.
type walker struct {
	loader Loader
	root   string
	report *Report
	faults *FaultList
}

// tag holds the parsed env tag and default tag of one field.
type tag struct {
	name     string
	skip     bool
	required bool
	secret   bool
	def      string
	hasDef   bool
	explicit bool
	unknown  string
}

func readTag(f reflect.StructField) tag {
	var t tag
	raw, ok := f.Tag.Lookup("env")
	t.explicit = ok
	if ok {
		parts := strings.Split(raw, ",")
		t.name = strings.TrimSpace(parts[0])
		if t.name == "-" {
			t.skip = true
		}
		for _, opt := range parts[1:] {
			switch strings.TrimSpace(opt) {
			case "required":
				t.required = true
			case "secret":
				t.secret = true
			default:
				t.unknown = strings.TrimSpace(opt)
			}
		}
	}
	t.def, t.hasDef = f.Tag.Lookup("default")
	return t
}

// walk fills every field of v. prefix joins to the environment variable name.
// path joins to the report path.
func (w *walker) walk(v reflect.Value, prefix, path string) {
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		tg := readTag(sf)
		if tg.skip {
			continue
		}
		name := tg.name
		if name == "" {
			name = screamingSnake(sf.Name)
		}
		if sf.Type.Kind() == reflect.Struct && sf.Type != urlType {
			if sf.Anonymous && !tg.explicit {
				// An embedded struct shares the prefix of its parent, because
				// Go promotes its fields.
				w.walk(v.Field(i), prefix, path)
				continue
			}
			w.walk(v.Field(i), prefix+name+"_", path+sf.Name+".")
			continue
		}
		w.leaf(v.Field(i), sf, tg, prefix+name, path+sf.Name)
	}
}

// leaf fills one field and appends its row to the report.
func (w *walker) leaf(v reflect.Value, sf reflect.StructField, tg tag, name, path string) {
	shape, ok := supported(sf.Type)
	if !ok {
		w.tagFault(name, path,
			fmt.Sprintf("%s.%s has type %s, which the loader does not support", w.root, path, sf.Type),
			fmt.Sprintf("Change %s.%s to a supported type, or mark it `env:\"-\"`", w.root, path))
		return
	}
	if tg.unknown != "" {
		w.tagFault(name, path,
			fmt.Sprintf("%s.%s carries the unknown env tag option %q", w.root, path, tg.unknown),
			fmt.Sprintf("Remove %q from the env tag on %s.%s", tg.unknown, w.root, path))
		return
	}
	if tg.required && tg.hasDef {
		w.tagFault(name, path,
			fmt.Sprintf("%s.%s is required and also carries a default", w.root, path),
			fmt.Sprintf("Remove `required` from the env tag on %s.%s, or remove its default tag", w.root, path))
		return
	}

	row := Field{Path: path, Name: name, Type: sf.Type.String(), Secret: tg.secret, Required: tg.required}

	switch raw, set := w.loader.lookup(name); {
	case set && raw == "" && tg.required:
		// An empty value is not a value. A required variable that holds one
		// must stop the process, because the code behind it reads a key or an
		// address that cannot work. See DX-8.
		row.Source = SourceAbsent
		w.faults.add(&Fault{
			Path:    w.root + "." + path,
			Name:    name,
			Message: fmt.Sprintf("%s is required and it holds an empty value", name),
			Repair:  fmt.Sprintf("Set %s in the environment, or run `avero doctor` to see every variable", name),
		})
	case set:
		row.Source = SourceEnv
		row.Value = show(raw, tg.secret)
		if !parse(v, raw) {
			w.faults.add(&Fault{
				Path:    w.root + "." + path,
				Name:    name,
				Message: fmt.Sprintf("%s received %q but it must be %s", name, show(raw, tg.secret), shape.name),
				Repair:  fmt.Sprintf("Set %s to %s such as `%s`", name, shape.name, shape.example),
			})
		}
	case tg.hasDef:
		row.Source = SourceDefault
		row.Value = show(tg.def, tg.secret)
		if !parse(v, tg.def) {
			w.tagFault(name, path,
				fmt.Sprintf("the default %q on %s.%s is not %s", show(tg.def, tg.secret), w.root, path, shape.name),
				fmt.Sprintf("Correct the default tag on %s.%s to %s such as `%s`", w.root, path, shape.name, shape.example))
		}
	case tg.required:
		row.Source = SourceAbsent
		w.faults.add(&Fault{
			Path:    w.root + "." + path,
			Name:    name,
			Message: fmt.Sprintf("%s is required but not set", name),
			Repair:  fmt.Sprintf("Set %s in the environment, or run `avero doctor` to see every variable", name),
		})
	default:
		row.Source = SourceAbsent
	}
	w.report.Fields = append(w.report.Fields, row)
}

func (w *walker) tagFault(name, path, message, repair string) {
	w.faults.add(&Fault{Path: w.root + "." + path, Name: name, Message: message, Repair: repair})
}

// show returns the text that Avero prints for a value. A secret never appears.
func show(raw string, secret bool) string {
	if secret && raw != "" {
		return Redacted
	}
	return raw
}
