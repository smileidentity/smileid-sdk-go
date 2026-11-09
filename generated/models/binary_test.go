package models

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pngHeader = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selfie.jpg")
	if err := os.WriteFile(path, []byte("jpeg-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := FromFile(path)
	if got := b.Filename(); got != "selfie.jpg" {
		t.Errorf("Filename() = %q, want selfie.jpg", got)
	}
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes(): %v", err)
	}
	if string(data) != "jpeg-bytes" {
		t.Errorf("Bytes() = %q", data)
	}
}

func TestFromFileMissing(t *testing.T) {
	b := FromFile(filepath.Join(t.TempDir(), "missing.jpg"))
	if _, err := b.Bytes(); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Bytes() error = %v, want os.ErrNotExist", err)
	}
}

func TestFromBytes(t *testing.T) {
	b := FromBytes([]byte("abc"), "doc.png")
	data, err := b.Bytes()
	if err != nil || string(data) != "abc" {
		t.Errorf("Bytes() = %q, %v", data, err)
	}
	if got := b.Filename(); got != "doc.png" {
		t.Errorf("Filename() = %q", got)
	}
}

func TestFromReader(t *testing.T) {
	b := FromReader(strings.NewReader("streamed"), "")
	data, err := b.Bytes()
	if err != nil || string(data) != "streamed" {
		t.Errorf("Bytes() = %q, %v", data, err)
	}
	if got := b.Filename(); got != "upload" {
		t.Errorf("Filename() = %q, want the upload default", got)
	}
}

func TestFromReaderError(t *testing.T) {
	if _, err := FromReader(failingReader{}, "x.jpg").Bytes(); err == nil {
		t.Error("Bytes() error = nil, want the reader's error")
	}
}

func TestBytesEmptyInput(t *testing.T) {
	data, err := (&BinaryInput{}).Bytes()
	if err != nil {
		t.Fatalf("Bytes(): %v", err)
	}
	if data == nil || len(data) != 0 {
		t.Errorf("Bytes() = %#v, want an empty non-nil slice", data)
	}
}

func TestWithContentType(t *testing.T) {
	orig := FromBytes([]byte("x"), "a.jpg")
	withCT := orig.WithContentType("image/webp")
	if withCT == orig {
		t.Fatal("WithContentType must return a copy")
	}
	if got := orig.ExplicitContentType(); got != "" {
		t.Errorf("original ExplicitContentType() = %q, want empty", got)
	}
	if got := withCT.ExplicitContentType(); got != "image/webp" {
		t.Errorf("ExplicitContentType() = %q", got)
	}
}

func TestContentTypeFor(t *testing.T) {
	tests := []struct {
		name  string
		input *BinaryInput
		field string
		data  []byte
		want  string
	}{
		{"override wins over png", FromBytes(pngHeader, "a.png").WithContentType("image/heic"), "document", pngHeader, "image/heic"},
		{"document png by extension", FromBytes([]byte("x"), "Front.PNG"), "document", []byte("x"), "image/png"},
		{"document_back png by magic", FromBytes(pngHeader, "back"), "document_back", pngHeader, "image/png"},
		{"document short data", FromBytes([]byte{0x89}, "front"), "document", []byte{0x89}, "image/jpeg"},
		{"document jpeg data", FromBytes([]byte("not-a-png-file"), "front.jpg"), "document", []byte("not-a-png-file"), "image/jpeg"},
		{"selfie is always jpeg", FromBytes(pngHeader, "selfie.png"), "selfie_image", pngHeader, "image/jpeg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.ContentTypeFor(tt.field, tt.data); got != tt.want {
				t.Errorf("ContentTypeFor(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestPNGMagicIsExact(t *testing.T) {
	notPNG := bytes.Repeat([]byte{0x89}, 8)
	if isPNG("file", notPNG) {
		t.Error("isPNG matched data without the PNG signature")
	}
}
