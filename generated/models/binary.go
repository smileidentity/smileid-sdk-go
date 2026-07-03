package models

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BinaryInput represents an image or file to be uploaded as a multipart part.
// Construct one with [FromFile], [FromBytes] or [FromReader].
type BinaryInput struct {
	path        string
	data        []byte
	reader      io.Reader
	name        string
	contentType string // explicit override; empty means auto-detect
}

// FromFile builds a BinaryInput that reads from the file at path when the
// request is sent. The filename part is derived from the path.
func FromFile(path string) *BinaryInput {
	return &BinaryInput{path: path, name: filepath.Base(path)}
}

// FromBytes builds a BinaryInput from an in-memory byte slice.
func FromBytes(b []byte, filename string) *BinaryInput {
	return &BinaryInput{data: b, name: filename}
}

// FromReader builds a BinaryInput from an io.Reader. The reader is consumed
// fully when the request is serialized.
func FromReader(r io.Reader, filename string) *BinaryInput {
	return &BinaryInput{reader: r, name: filename}
}

// WithContentType returns a copy of the input with an explicit content type,
// overriding the default detection. The override always wins.
func (b *BinaryInput) WithContentType(ct string) *BinaryInput {
	nb := *b
	nb.contentType = ct
	return &nb
}

// Filename returns the filename used for the multipart part.
func (b *BinaryInput) Filename() string {
	if b.name != "" {
		return b.name
	}
	return "upload"
}

// Bytes materializes the input into a byte slice.
func (b *BinaryInput) Bytes() ([]byte, error) {
	switch {
	case b.data != nil:
		return b.data, nil
	case b.path != "":
		return os.ReadFile(b.path)
	case b.reader != nil:
		return io.ReadAll(b.reader)
	default:
		return []byte{}, nil
	}
}

// ExplicitContentType returns the caller-provided content type override, if any.
func (b *BinaryInput) ExplicitContentType() string { return b.contentType }

// ContentTypeFor resolves the content type for a part with the given field
// name and materialized data. PNG detection (by extension or magic bytes)
// applies only to the document and document_back fields; every other image
// field is image/jpeg. An explicit override always wins.
func (b *BinaryInput) ContentTypeFor(field string, data []byte) string {
	if b.contentType != "" {
		return b.contentType
	}
	if field == "document" || field == "document_back" {
		if isPNG(b.name, data) {
			return "image/png"
		}
	}
	return "image/jpeg"
}

func isPNG(name string, data []byte) bool {
	if strings.HasSuffix(strings.ToLower(name), ".png") {
		return true
	}
	pngMagic := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	return len(data) >= len(pngMagic) && bytes.Equal(data[:len(pngMagic)], pngMagic)
}
