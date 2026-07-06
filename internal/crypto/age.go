package crypto

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

func EncryptFile(srcPath, destPath, recipient string) error {
	r, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	cleanup := func() {
		dst.Close()
		os.Remove(destPath)
	}

	w, err := age.Encrypt(dst, r)
	if err != nil {
		cleanup()
		return fmt.Errorf("encrypt: %w", err)
	}

	if _, err := io.Copy(w, src); err != nil {
		w.Close()
		cleanup()
		return fmt.Errorf("copy: %w", err)
	}

	if err := w.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close encrypt writer: %w", err)
	}

	return dst.Close()
}

func DecryptFile(srcPath, destPath, identity string) error {
	id, err := age.ParseX25519Identity(identity)
	if err != nil {
		return fmt.Errorf("invalid identity: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dst.Close()

	r, err := age.Decrypt(src, id)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	if _, err := io.Copy(dst, r); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}

func GenerateKey() (identity string, recipient string, err error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	return id.String(), id.Recipient().String(), nil
}

func ParseIdentity(identity string) (*age.X25519Identity, error) {
	return age.ParseX25519Identity(identity)
}
