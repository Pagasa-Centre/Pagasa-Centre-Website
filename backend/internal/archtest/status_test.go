package archtest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestStorageDoesNotDecideStatus(t *testing.T) {
	backend := backendRoot(t)
	internal := filepath.Join(backend, "internal")
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`billing_status\s*=\s*'`),
		regexp.MustCompile(`payment_status\s*=\s*'`),
		regexp.MustCompile(`CASE WHEN paid_in_full`),
	}

	var violations []string
	err := walkGoFiles(internal, func(path string) {
		rel := relInternal(path, backend)
		if !strings.Contains(rel, "/storage/") {
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
	if len(violations) > 0 {
		t.Fatalf("storage must not hard-code status transitions:\n%s", strings.Join(violations, "\n"))
	}
}
