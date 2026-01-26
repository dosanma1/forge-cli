//go:build windows
// +build windows

// Package xos provides cross-platform atomic file operations.
// On Windows, we use a fallback approach since atomic rename across
// drives is not always possible.
package xos

import (
	"io"
	"os"
	"path/filepath"
)

// WriteFile writes data to the named file.
// On Windows, this uses a temp file + rename approach within the same directory.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	// Create temp file in the same directory as the target
	dir := filepath.Dir(filename)
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()

	// Clean up temp file on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tempName)
		}
	}()

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	// Sync to ensure data is on disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	// Set permissions
	if err := os.Chmod(tempName, perm); err != nil {
		return err
	}

	// On Windows, we need to remove the target first if it exists
	if _, err := os.Stat(filename); err == nil {
		if err := os.Remove(filename); err != nil {
			return err
		}
	}

	// Rename temp file to target
	if err := os.Rename(tempName, filename); err != nil {
		return err
	}

	success = true
	return nil
}

// WriteFileTemp writes data to a temp file and then renames it atomically.
func WriteFileTemp(filename string, data []byte, perm os.FileMode, tempDir string) error {
	// On Windows, use the specified temp dir
	tempFile, err := os.CreateTemp(tempDir, ".tmp-*")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()

	// Clean up temp file on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tempName)
		}
	}()

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tempName, perm); err != nil {
		return err
	}

	// On Windows, we need to remove the target first if it exists
	if _, err := os.Stat(filename); err == nil {
		if err := os.Remove(filename); err != nil {
			return err
		}
	}

	if err := os.Rename(tempName, filename); err != nil {
		return err
	}

	success = true
	return nil
}

// WriteReader writes data from a reader to the named file atomically.
func WriteReader(filename string, r io.Reader, perm os.FileMode) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return WriteFile(filename, data, perm)
}

// CreateDir creates a directory and all necessary parents.
func CreateDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFileWithBackup writes data to a file, creating a backup of the original.
func WriteFileWithBackup(filename string, data []byte, perm os.FileMode) error {
	if _, err := os.Stat(filename); err == nil {
		backupPath := filename + ".bak"
		original, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		if err := WriteFile(backupPath, original, perm); err != nil {
			return err
		}
	}
	return WriteFile(filename, data, perm)
}

// PendingFile represents a file that will be written atomically.
type PendingFile struct {
	tempFile *os.File
	tempName string
	path     string
	perm     os.FileMode
}

// NewPendingFile creates a new pending file for atomic writing.
func NewPendingFile(filename string) (*PendingFile, error) {
	dir := filepath.Dir(filename)
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return nil, err
	}
	return &PendingFile{
		tempFile: tempFile,
		tempName: tempFile.Name(),
		path:     filename,
		perm:     0644,
	}, nil
}

// Write writes data to the pending file.
func (p *PendingFile) Write(data []byte) (int, error) {
	return p.tempFile.Write(data)
}

// WriteString writes a string to the pending file.
func (p *PendingFile) WriteString(s string) (int, error) {
	return p.tempFile.WriteString(s)
}

// Chmod changes the file mode of the pending file.
func (p *PendingFile) Chmod(perm os.FileMode) error {
	p.perm = perm
	return nil
}

// CloseAtomically completes the write by atomically renaming the temp file.
func (p *PendingFile) CloseAtomically() error {
	if err := p.tempFile.Sync(); err != nil {
		p.tempFile.Close()
		os.Remove(p.tempName)
		return err
	}

	if err := p.tempFile.Close(); err != nil {
		os.Remove(p.tempName)
		return err
	}

	if err := os.Chmod(p.tempName, p.perm); err != nil {
		os.Remove(p.tempName)
		return err
	}

	// Remove target if exists
	if _, err := os.Stat(p.path); err == nil {
		if err := os.Remove(p.path); err != nil {
			os.Remove(p.tempName)
			return err
		}
	}

	return os.Rename(p.tempName, p.path)
}

// Cleanup discards the pending file without writing.
func (p *PendingFile) Cleanup() {
	p.tempFile.Close()
	os.Remove(p.tempName)
}

// Path returns the target path of the pending file.
func (p *PendingFile) Path() string {
	return p.path
}

// Symlink creates a symbolic link.
// Note: On Windows, this requires elevated privileges or developer mode.
func Symlink(oldname, newname string) error {
	// Remove existing symlink if present
	if _, err := os.Lstat(newname); err == nil {
		if err := os.Remove(newname); err != nil {
			return err
		}
	}
	return os.Symlink(oldname, newname)
}

// TempDir returns the temporary directory used for atomic operations.
func TempDir(filename string) string {
	return filepath.Dir(filename)
}

// CopyFile copies a file atomically.
func CopyFile(src, dst string, perm os.FileMode) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, content, perm)
}
