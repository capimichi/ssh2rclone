package helper

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AtomicWriteFile safely writes data to target path by writing to a temporary file
// in the same directory and renaming it over the destination.
func AtomicWriteFile(destPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, "ssh2rclone-tmp-*.conf")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()

	defer func() {
		// Clean up temp file if rename didn't happen
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write data to temp file: %w", err)
	}

	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmpName, destPath, err)
	}

	return nil
}

// CreateBackup copies destPath to destPath.bak if destPath exists.
func CreateBackup(destPath string) (string, error) {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return "", nil // Nothing to back up
	}

	bakPath := destPath + ".bak"
	src, err := os.Open(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source for backup: %w", err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat source: %w", err)
	}

	dst, err := os.OpenFile(bakPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy data to backup: %w", err)
	}

	return bakPath, nil
}
