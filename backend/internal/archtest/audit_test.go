package archtest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestOnlyUseCasesRecordAudit(t *testing.T) {
	backend := backendRoot(t)
	roots := []string{
		filepath.Join(backend, "internal", "api"),
		filepath.Join(backend, "internal", "middleware"),
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`admin\.Audit\(`),
		regexp.MustCompile(`\.Record\(`),
	}

	var violations []string
	for _, root := range roots {
		err := walkGoFiles(root, func(path string) {
			rel := relInternal(path, backend)
			if rel == "" {
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(string(data), "\n") {
				for _, re := range patterns {
					if re.MatchString(line) {
						violations = append(violations, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("admin audit must only be recorded from internal/campadmin:\n%s", strings.Join(violations, "\n"))
	}
}
