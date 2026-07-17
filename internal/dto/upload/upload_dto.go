// Package upload holds request/response DTOs for the file-upload module.
package upload

// ChunkRequest is the parsed metadata of a single chunk POST (the binary chunk
// itself is carried separately as a multipart file).
type ChunkRequest struct {
	SessionID   string
	ChunkIndex  int
	TotalChunks int
	FileName    string
	Sha256      string
	FileSize    int64
	ChunkSize   int64
	ForceUpload bool
}

// ChunkResult is returned for each chunk request.
type ChunkResult struct {
	SessionID      string `json:"session_id"`
	ChunkIndex     int    `json:"chunk_index"`
	TotalChunks    int    `json:"total_chunks"`
	UploadComplete bool   `json:"upload_complete"`
	FileName       string `json:"file_name,omitempty"`
	ObjectPath     string `json:"object_path,omitempty"`
	PreviewURL     string `json:"preview_url,omitempty"`
	Sha256         string `json:"sha256,omitempty"`
}
