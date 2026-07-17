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

func TestValidateSessionID(t *testing.T) {
	ok := []string{"7E94247A-EE64-4D81-90F9-A8077238FAF9", "abc12345", "a-b-c-1-2-3-4-5"}
	for _, s := range ok {
		if err := validateSessionID(s); err != nil {
			t.Errorf("expected %q valid, got %v", s, err)
		}
	}
	bad := []string{"", "short", "../evil", "sess/ion", "has space", "sess;drop"}
	for _, s := range bad {
		if err := validateSessionID(s); err == nil {
			t.Errorf("expected %q rejected", s)
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
