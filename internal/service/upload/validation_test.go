package upload

import "testing"

func TestValidateFileName(t *testing.T) {
	ok := []string{"laporan tahunan.pdf", "Doc_2026 (final)[v1].pdf", "a-b.c.pdf"}
	for _, n := range ok {
		if err := validateFileName(n); err != nil {
			t.Errorf("expected %q valid, got %v", n, err)
		}
	}
	bad := []string{"../etc/passwd.pdf", "a/b.pdf", "a\\b.pdf", "foo;rm.pdf", "shell$.pdf"}
	for _, n := range bad {
		if err := validateFileName(n); err == nil {
			t.Errorf("expected %q rejected", n)
		}
	}
}

func TestValidatePDFExtension(t *testing.T) {
	if err := validatePDFExtension("report.pdf"); err != nil {
		t.Errorf("report.pdf should pass: %v", err)
	}
	bad := []string{"report.docx", "image.png", "noext", "malware.exe.pdf", "script.sh.pdf", "page.html.pdf"}
	for _, n := range bad {
		if err := validatePDFExtension(n); err == nil {
			t.Errorf("expected %q rejected", n)
		}
	}
}
