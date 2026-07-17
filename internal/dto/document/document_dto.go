// Package document holds response DTOs for the document (uploaded file) module.
package document

import "time"

// DocumentResponse is the public representation of an uploaded document
// (sourced from a completed upload log).
type DocumentResponse struct {
	ID               uint      `json:"id" example:"7"`
	FileName         string    `json:"file_name" example:"laporan.pdf"`
	FileSize         int64     `json:"file_size" example:"13000007"`
	TotalChunks      int       `json:"total_chunks" example:"3"`
	Sha256           string    `json:"sha256"`
	OrganizationCode string    `json:"organization_code" example:"pln"`
	UploadedBy       uint      `json:"uploaded_by" example:"2"`
	ObjectPath       string    `json:"object_path" example:"uploads/pln/2/xxx.pdf"`
	PreviewURL       string    `json:"preview_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
