package assets

import "testing"

func TestBareSpecifierReadsTheNameOfAPackage(t *testing.T) {
	for path, want := range map[string]string{
		"/react@19.2.0/es2022/react.mjs":                      "react",
		"/react@^19.1.1?target=es2022":                        "react",
		"/react@^19.1.1/jsx-runtime?target=es2022":            "react/jsx-runtime",
		"/@tanstack/query-core@5.90.2/es2022/query-core.mjs":  "@tanstack/query-core",
		"/@tanstack/react-query@5.90.2/es2022/react-query.js": "@tanstack/react-query",
		"/scheduler@0.27.0/es2022/scheduler.mjs":              "scheduler",
		"/assets/app.css":                                     "",
		"/":                                                   "",
	} {
		if got := bareSpecifier(path); got != want {
			t.Fatalf("bareSpecifier(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestVendorSpecifiersRewritesTheImportsOfAModule(t *testing.T) {
	body := []byte(`import"/react@19.2.0/es2022/react.mjs";import{jsx as e}from"/react@^19.1.1/jsx-runtime?target=es2022";export*from"/react-dom@19.2.0/es2022/client.bundle.mjs";const url="/assets/app.css";`)
	got := string(vendorSpecifiers(body))
	want := `import"react";import{jsx as e}from"react/jsx-runtime";export*from"react-dom";const url="/assets/app.css";`
	if got != want {
		t.Fatalf("vendorSpecifiers wrote\n%s\nwant\n%s", got, want)
	}
}
