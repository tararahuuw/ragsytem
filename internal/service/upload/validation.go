package upload

import (
	"net/http"
	"regexp"
	"strings"
)

// Whitelist of allowed filename characters (ported from elArch).
var fileNameRe = regexp.MustCompile(`^[a-zA-Z0-9._ \-\(\)\[\]]+$`)

// sessionID must be UUID-like (alphanumeric + dash) since it is used verbatim
// as part of the object storage key.
var sessionIDRe = regexp.MustCompile(`^[a-zA-Z0-9\-]{8,64}$`)

// validateSessionID guards the object key against injection / traversal.
func validateSessionID(id string) error {
	if !sessionIDRe.MatchString(id) {
		return newErr("INVALID_SESSION", http.StatusBadRequest, "sessionId tidak valid")
	}
	return nil
}

// Dangerous inner extensions to reject in double-extension names (e.g. "x.exe.pdf").
var dangerousExts = []string{
	".exe", ".sh", ".bat", ".cmd", ".php", ".js", ".jar", ".bin",
	".msi", ".com", ".scr", ".ps1", ".html", ".htm", ".svg", ".dll",
}

// validateFileName rejects path-traversal and disallowed characters.
func validateFileName(name string) error {
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return newErr("INVALID_FILENAME", http.StatusBadRequest, "Nama file mengandung pola path yang tidak diizinkan")
	}
	if !fileNameRe.MatchString(name) {
		return newErr("INVALID_FILENAME", http.StatusBadRequest, "Nama file mengandung karakter spesial yang tidak diizinkan")
	}
	return nil
}

// validatePDFExtension enforces a single .pdf extension (MVP is PDF-only) and
// blocks double-extension tricks.
func validatePDFExtension(name string) error {
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".pdf") {
		return newErr("INVALID_FILE_TYPE", http.StatusBadRequest, "Hanya file PDF yang diizinkan")
	}
	base := strings.TrimSuffix(lower, ".pdf")
	for _, ext := range dangerousExts {
		if strings.HasSuffix(base, ext) {
			return newErr("INVALID_FILE_TYPE", http.StatusBadRequest, "Nama file mengandung ekstensi ganda yang tidak diizinkan")
		}
	}
	return nil
}
