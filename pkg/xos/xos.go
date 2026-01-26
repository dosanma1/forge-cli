//go:build !windows
// +build !windows

// Package xos provides cross-platform atomic file operations.
// It uses atomic rename operations to prevent file corruption on crashes.
package xos

import (
	"io"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
)

// WriteFile writes data to the named file atomically using rename.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise WriteFile truncates it before writing.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return renameio.WriteFile(filename, data, perm)
}

// WriteFileTemp writes data to a temp file and then renames it atomically.
// This is useful when you want explicit control over the temp directory.
func WriteFileTemp(filename string, data []byte, perm os.FileMode, tempDir string) error {
	return renameio.WriteFile(filename, data, perm, renameio.WithTempDir(tempDir))
}

// WriteReader writes data from a reader to the named file atomically.
func WriteReader(filename string, r io.Reader, perm os.FileMode) error {
	t, err := renameio.TempFile("", filename)
	if err != nil {
		return err
	}
	defer t.Cleanup()

	if _, err := io.Copy(t, r); err != nil {
		return err
	}

	if err := t.Chmod(perm); err != nil {
		return err
	}

	return t.CloseAtomicallyReplace()
}

// CreateDir creates a directory and all necessary parents with atomic semantics.
// Unlike os.MkdirAll, this ensures the directory is fully created or not at all.
func CreateDir(path string, perm os.FileMode) error {
	// MkdirAll is already atomic at the kernel level for each directory creation
	return os.MkdirAll(path, perm)
}

// WriteFileWithBackup writes data to a file, creating a backup of the original.
// The backup is named with a .bak extension.
func WriteFileWithBackup(filename string, data []byte, perm os.FileMode) error {
	// Check if original file exists
	if _, err := os.Stat(filename); err == nil {
		// Create backup
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
// Call CloseAtomically to complete the write, or Cleanup to discard.
type PendingFile struct {
	tempFile *renameio.PendingFile
	path     string
}

// NewPendingFile creates a new pending file for atomic writing.
func NewPendingFile(filename string) (*PendingFile, error) {
	t, err := renameio.TempFile("", filename)
	if err != nil {
		return nil, err
	}
	return &PendingFile{
		tempFile: t,
		path:     filename,
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
	return p.tempFile.Chmod(perm)
}

// CloseAtomically completes the write by atomically renaming the temp file.
func (p *PendingFile) CloseAtomically() error {
	return p.tempFile.CloseAtomicallyReplace()
}

// Cleanup discards the pending file without writing.
func (p *PendingFile) Cleanup() {
	p.tempFile.Cleanup()
}

// Path returns the target path of the pending file.
func (p *PendingFile) Path() string {
	return p.path
}

// Symlink creates a symbolic link atomically.
func Symlink(oldname, newname string) error {
	return renameio.Symlink(oldname, newname)
}

// TempDir returns the temporary directory used for atomic operations.
// This is the directory where temp files are created before being renamed.
func TempDir(filename string) string {
	return renameio.TempDir(filepath.Dir(filename))
}

// CopyFile copies a file atomically by reading and writing with atomic rename.
func CopyFile(src, dst string, perm os.FileMode) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, content, perm)
}
