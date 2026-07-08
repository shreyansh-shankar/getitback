package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
)

type Downloader struct {
	exec executor.Executor
}

func New(exec executor.Executor) Downloader {
	return Downloader{exec: exec}
}

func (d Downloader) Get(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	out, err := os.Create(dest + ".tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		os.Remove(dest + ".tmp")
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(dest + ".tmp")
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(dest + ".tmp")
		return fmt.Errorf("write body: %w", err)
	}

	out.Close()
	return os.Rename(dest+".tmp", dest)
}

func (d Downloader) VerifyChecksum(path, expectedSHA256 string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	return got == expectedSHA256, nil
}

func (d Downloader) GetAndVerify(url, dest, sha256sum string) error {
	if err := d.Get(url, dest); err != nil {
		return err
	}
	ok, err := d.VerifyChecksum(dest, sha256sum)
	if err != nil {
		return fmt.Errorf("verify checksum: %w", err)
	}
	if !ok {
		os.Remove(dest)
		return fmt.Errorf("checksum mismatch for %s", dest)
	}
	return nil
}
