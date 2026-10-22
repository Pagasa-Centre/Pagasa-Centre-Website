package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

var (
	httpPackages = map[string]bool{
		"internal/api":        true,
		"internal/http":       true,
		"internal/middleware": true,
		"internal/testhelper": true,
		"internal/archtest":   true,
	}
)

func walkGoFiles(root string, fn func(path string)) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fn(path)
		return nil
	})
}

func relInternal(path, backend string) string {
	rel, err := filepath.Rel(backend, path)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "internal/") {
		return ""
	}
	return rel
}

func importPaths(fset *token.FileSet, path string) []string {
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var out []string
	for _, spec := range f.Imports {
		p := strings.Trim(spec.Path.Value, `"`)
		out = append(out, p)
	}
	return out
}

func pgxImportForbidden(imp string) bool {
	return imp == "github.com/jackc/pgx/v5" || strings.HasPrefix(imp, "github.com/jackc/pgx/")
}

func storageImportForbidden(imp string) bool {
	return strings.HasPrefix(imp, "pagasacentre/backend/internal/") && strings.HasSuffix(imp, "/storage")
}

func exemptFromPgxRule(rel string) bool {
	if strings.Contains(rel, "/storage/") {
		return true
	}
	switch {
	case rel == "internal/testhelper/db.go",
		strings.HasPrefix(rel, "internal/testhelper/") && strings.HasSuffix(rel, "_test.go"),
		strings.HasPrefix(rel, "internal/billing/billingtest/"):
		return true
	case strings.HasPrefix(rel, "internal/archtest/"):
		return true
	default:
		return false
	}
}

func TestOnlyStorageTouchesPgx(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()

	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" || exemptFromPgxRule(rel) {
			return
		}
		for _, imp := range importPaths(fset, path) {
			if pgxImportForbidden(imp) {
				violations = append(violations, rel+" imports "+imp)
			}
			if storageImportForbidden(imp) {
				violations = append(violations, rel+" imports "+imp)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("only */storage and testhelper may import pgx or */storage:\n%s", strings.Join(violations, "\n"))
	}
}

func TestServicesDoNotImportHTTPErrorPackage(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()

	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" {
			return
		}
		for prefix := range httpPackages {
			if strings.HasPrefix(rel, prefix+"/") || rel == prefix+".go" {
				return
			}
		}
		if !strings.HasPrefix(rel, "internal/") {
			return
		}
		// allow only non-http internal packages check — skip archtest itself handled above
		parts := strings.Split(strings.TrimPrefix(rel, "internal/"), "/")
		if len(parts) == 0 {
			return
		}
		top := "internal/" + parts[0]
		if httpPackages[top] {
			return
		}

		for _, imp := range importPaths(fset, path) {
			if imp == "pagasacentre/backend/pkg/commonlibrary/errors" {
				violations = append(violations, rel+" imports "+imp)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("forbidden commonlibrary/errors imports:\n%s", strings.Join(violations, "\n"))
	}
}

func TestDomainPackagesOnlyImportStdlibAndApperr(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()

	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if !strings.Contains(rel, "/domain/") {
			return
		}
		for _, imp := range importPaths(fset, path) {
			if !strings.Contains(imp, "/") {
				continue // stdlib
			}
			if imp == "pagasacentre/backend/pkg/commonlibrary/apperr" {
				continue
			}
			violations = append(violations, rel+" imports "+imp)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("domain packages may only import stdlib and apperr:\n%s", strings.Join(violations, "\n"))
	}
}

func TestApperrImportsOnlyStdlib(t *testing.T) {
	backend := backendRoot(t)
	apperrDir := filepath.Join(backend, "pkg", "commonlibrary", "apperr")
	fset := token.NewFileSet()

	var violations []string
	err := walkGoFiles(apperrDir, func(path string) {
		rel, _ := filepath.Rel(backend, path)
		rel = filepath.ToSlash(rel)
		for _, imp := range importPaths(fset, path) {
			if strings.Contains(imp, "/") {
				violations = append(violations, rel+" imports "+imp)
			}
			if imp == "net/http" {
				violations = append(violations, rel+" imports net/http")
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("apperr must be stdlib-only:\n%s", strings.Join(violations, "\n"))
	}
}

func TestStripeSDKOnlyInAdapters(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()
	const stripeImport = "github.com/stripe/stripe-go"
	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" {
			return
		}
		if strings.HasPrefix(rel, "internal/stripe/") {
			return
		}
		if rel == "internal/api/payment/handler.go" {
			return
		}
		for _, imp := range importPaths(fset, path) {
			if imp == stripeImport || strings.HasPrefix(imp, stripeImport+"/") {
				violations = append(violations, rel+" imports "+imp)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("stripe SDK only in internal/stripe and api/payment handler:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCoreDoesNotImportNotificationAdapters(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()
	prefixes := []string{
		"internal/registration/",
		"internal/billing/",
		"internal/payment/",
		"internal/campadmin/",
		"internal/auth/",
		"internal/notify/",
	}
	forbidden := []string{
		"pagasacentre/backend/internal/email",
		"pagasacentre/backend/internal/sheets",
	}
	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" || strings.HasSuffix(rel, "_test.go") {
			return
		}
		ok := false
		for _, p := range prefixes {
			if strings.HasPrefix(rel, p) {
				ok = true
				break
			}
		}
		if !ok {
			return
		}
		for _, imp := range importPaths(fset, path) {
			for _, f := range forbidden {
				if imp == f {
					violations = append(violations, rel+" imports "+imp)
				}
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("core must not import notification adapters:\n%s", strings.Join(violations, "\n"))
	}
}

func TestNotifyImportsOnlyStdlibAndDomain(t *testing.T) {
	backend := backendRoot(t)
	notifyDir := filepath.Join(backend, "internal", "notify")
	fset := token.NewFileSet()
	var violations []string
	err := walkGoFiles(notifyDir, func(path string) {
		rel, _ := filepath.Rel(backend, path)
		rel = filepath.ToSlash(rel)
		for _, imp := range importPaths(fset, path) {
			if !strings.Contains(imp, "/") {
				continue
			}
			if imp == "pagasacentre/backend/internal/registration/domain" {
				continue
			}
			violations = append(violations, rel+" imports "+imp)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("notify may only import stdlib and domain:\n%s", strings.Join(violations, "\n"))
	}
}

func TestBillingAndPaymentDoNotImportRegistrationService(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()
	const forbidden = "pagasacentre/backend/internal/registration"
	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" {
			return
		}
		if !strings.HasPrefix(rel, "internal/billing/") && !strings.HasPrefix(rel, "internal/payment/") {
			return
		}
		for _, imp := range importPaths(fset, path) {
			if imp == forbidden {
				violations = append(violations, rel+" imports "+imp)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("billing/payment must not import registration service package:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCoreIdentifiersAreVendorNeutral(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	fset := token.NewFileSet()
	prefixes := []string{
		"internal/registration/",
		"internal/billing/",
		"internal/payment/",
		"internal/campadmin/",
	}
	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if rel == "" {
			return
		}
		ok := false
		for _, p := range prefixes {
			if strings.HasPrefix(rel, p) {
				ok = true
				break
			}
		}
		if !ok {
			return
		}
		if strings.Contains(rel, "/storage/") || strings.Contains(rel, "/billingtest/") {
			return
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		checkNeutralNames(f, rel, &violations)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("vendor-neutral core identifiers:\n%s", strings.Join(violations, "\n"))
	}
}

func checkNeutralNames(f *ast.File, rel string, violations *[]string) {
	const bad = "stripe"
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if strings.Contains(strings.ToLower(x.Name.Name), bad) {
				*violations = append(*violations, rel+": type "+x.Name.Name)
			}
		case *ast.StructType:
			for _, fld := range x.Fields.List {
				for _, nm := range fld.Names {
					if strings.Contains(strings.ToLower(nm.Name), bad) {
						*violations = append(*violations, rel+": field "+nm.Name)
					}
				}
			}
		case *ast.FuncDecl:
			if x.Recv == nil && strings.Contains(strings.ToLower(x.Name.Name), bad) {
				*violations = append(*violations, rel+": func "+x.Name.Name)
			}
			if x.Recv != nil {
				for _, r := range x.Recv.List {
					for _, nm := range r.Names {
						if strings.Contains(strings.ToLower(nm.Name), bad) {
							*violations = append(*violations, rel+": method recv "+nm.Name)
						}
					}
				}
			}
		case *ast.InterfaceType:
			for _, m := range x.Methods.List {
				for _, nm := range m.Names {
					if strings.Contains(strings.ToLower(nm.Name), bad) {
						*violations = append(*violations, rel+": iface method "+nm.Name)
					}
				}
			}
		}
		return true
	})
}
