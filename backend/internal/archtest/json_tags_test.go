package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var jsonTagAllowlist = map[string]bool{
	"internal/middleware/admin_auth.go": true,
	"internal/email/resend.go":          true,
	"internal/sermon/youtube.go":        true,
	"internal/campadmin/audit_meta.go":  true,
}

func TestJSONTagsOnlyInAPILayer(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()

	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" {
			return
		}
		if strings.HasPrefix(rel, "internal/api/") {
			return
		}
		if jsonTagAllowlist[rel] {
			return
		}

		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				if _, ok := tag.Lookup("json"); ok {
					violations = append(violations, rel+" has json tag on struct field")
				}
			}
			return true
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("json tags outside internal/api and allowlist:\n%s", strings.Join(violations, "\n"))
	}
}
