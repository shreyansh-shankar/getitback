package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func createTestArchive(t *testing.T, entries map[string]string) string {
	t.Helper()
	f, err := os.CreateTemp("", "test-archive-*.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)

	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Size:     int64(len(content)),
			Mode:     0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	zw.Close()
	return f.Name()
}

func TestWalkEntries(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"manifest.json": `{"version":"1"}`,
		"images/a.tar":  "image-a-data",
		"images/b.tar":  "image-b-data",
		"configs/daemon.json": `{"debug":true}`,
	})
	defer os.Remove(path)

	var names []string
	err := WalkEntries(path, func(hdr *tar.Header, r io.Reader) error {
		names = append(names, hdr.Name)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 4 {
		t.Fatalf("expected 4 entries, got %d: %v", len(names), names)
	}
}

func TestWalkEntriesData(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"test.txt": "hello world",
	})
	defer os.Remove(path)

	err := WalkEntries(path, func(hdr *tar.Header, r io.Reader) error {
		if hdr.Name != "test.txt" {
			t.Fatalf("unexpected name: %s", hdr.Name)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if string(data) != "hello world" {
			t.Fatalf("expected 'hello world', got %q", string(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestExtractEntry(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"manifest.json": `{"version":"1"}`,
		"images/img.tar": "image-data",
	})
	defer os.Remove(path)

	dest := t.TempDir()
	err := ExtractEntry(path, "images/img.tar", dest)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "images/img.tar"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "image-data" {
		t.Fatalf("expected 'image-data', got %q", string(data))
	}
}

func TestExtractEntryNotFound(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"test.txt": "hello",
	})
	defer os.Remove(path)

	err := ExtractEntry(path, "nonexistent.txt", t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
	if !strings.Contains(err.Error(), "file does not exist") {
		t.Fatalf("expected file not exist error, got %v", err)
	}
}

func TestExtractMatching(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"images/a.tar":      "image-a",
		"images/b.tar":      "image-b",
		"configs/test.conf": "config-data",
	})
	defer os.Remove(path)

	dest := t.TempDir()
	err := ExtractMatching(path, "images/", dest)
	if err != nil {
		t.Fatal(err)
	}

	// Check only images were extracted
	entries, _ := os.ReadDir(filepath.Join(dest, "images"))
	if len(entries) != 2 {
		t.Fatalf("expected 2 files in images/, got %d", len(entries))
	}
	// configs should NOT have been extracted
	if _, err := os.Stat(filepath.Join(dest, "configs")); err == nil {
		t.Fatal("configs should not have been extracted")
	}
}

func TestExtractWithPrefix(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"images/a.tar":     "image-a",
		"images/sub/b.tar": "image-b",
	})
	defer os.Remove(path)

	dest := t.TempDir()
	err := ExtractWithPrefix(path, "images", dest)
	if err != nil {
		t.Fatal(err)
	}

	// Check prefix was stripped — should be at root of dest
	data, err := os.ReadFile(filepath.Join(dest, "a.tar"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "image-a" {
		t.Fatalf("expected 'image-a', got %q", string(data))
	}
}

func TestOpenReader(t *testing.T) {
	path := createTestArchive(t, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	})
	defer os.Remove(path)

	r, err := OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var contents []string
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(r)
		contents = append(contents, string(data))
		_ = hdr
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(contents))
	}
	if contents[0] != "content1" || contents[1] != "content2" {
		t.Fatalf("unexpected contents: %v", contents)
	}
}

func TestOpenReaderMultiplePasses(t *testing.T) {
	// Verify the same archive can be read twice (for two-pass streaming)
	path := createTestArchive(t, map[string]string{
		"images/a.tar": "image-a-data",
		"configs/cfg":  "config-data",
	})
	defer os.Remove(path)

	// Pass 1: count images
	imgCount := 0
	r1, _ := OpenReader(path)
	for {
		hdr, err := r1.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			r1.Close()
			t.Fatal(err)
		}
		if strings.HasPrefix(hdr.Name, "images/") {
			imgCount++
		}
	}
	r1.Close()

	if imgCount != 1 {
		t.Fatalf("expected 1 image, got %d", imgCount)
	}

	// Pass 2: extract configs
	dest := t.TempDir()
	r2, _ := OpenReader(path)
	for {
		hdr, err := r2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			r2.Close()
			t.Fatal(err)
		}
		if strings.HasPrefix(hdr.Name, "configs/") {
			WriteEntry(hdr, r2, dest)
		}
	}
	r2.Close()

	data, err := os.ReadFile(filepath.Join(dest, "configs/cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "config-data" {
		t.Fatalf("expected 'config-data', got %q", string(data))
	}
}

func TestWriteEntry(t *testing.T) {
	dest := t.TempDir()

	hdr := &tar.Header{
		Name:     "subdir/test.txt",
		Typeflag: tar.TypeReg,
		Size:     int64(len("hello")),
		Mode:     0644,
	}
	r := io.NopCloser(bytes.NewReader([]byte("hello")))
	if err := WriteEntry(hdr, r, dest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "subdir/test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(data))
	}
}
