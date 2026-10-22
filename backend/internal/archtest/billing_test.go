package archtest

import (
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestBillingGroupsDoNotImportSiblings(t *testing.T) {
	backend := backendRoot(t)
	billingRoot := filepath.Join(backend, "internal", "billing")
	groups := []string{"allocation", "invoicing", "camperedit", "reconcile", "sweep", "sheetsync"}
	fset := token.NewFileSet()

	var violations []string
	for _, g := range groups {
		prefix := filepath.Join(billingRoot, g)
		err := walkGoFiles(prefix, func(path string) {
			rel := relInternal(path, backend)
			if rel == "" || strings.Contains(rel, "/billingtest/") {
				return
			}
			for _, imp := range importPaths(fset, path) {
				for _, other := range groups {
					if other == g {
						continue
					}
					sibling := "pagasacentre/backend/internal/billing/" + other
					if imp == sibling || strings.HasPrefix(imp, sibling+"/") {
						violations = append(violations, rel+" imports "+imp)
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("billing sibling imports:\n%s", strings.Join(violations, "\n"))
	}
}
