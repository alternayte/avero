package uifiles_test

import (
	"strings"
	"testing"

	"github.com/alternayte/avero/internal/cli/uifiles"
)

func TestTheToasterHoldsTheBasecoatMarkup(t *testing.T) {
	body := string(uifiles.Toaster())
	for _, want := range []string{
		"package ui",
		"id=\"toaster\"",
		"class=\"toaster\"",
		"data-align=\"end\"",
		"data-category",
		"data-toast-cancel",
		"aria-atomic",
		"templ Toasts()",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("the component holds no %q", want)
		}
	}
}
