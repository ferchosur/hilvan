package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// UnsafePathError reports a symlink, a non-regular file or an oversized file (SR-006/SR-007).
type UnsafePathError struct{ Msg string }

func (e *UnsafePathError) Error() string { return e.Msg }

func unsafePath(format string, args ...any) error {
	return &UnsafePathError{Msg: fmt.Sprintf(format, args...)}
}

// IsUnsafePath reports whether err is an UnsafePathError.
func IsUnsafePath(err error) bool {
	var u *UnsafePathError
	return errors.As(err, &u)
}

// ReadTextSafe reads a regular file. found is false when it does not exist.
// Symlinks (O_NOFOLLOW where available, lstat elsewhere) and files above maxBytes are rejected.
func ReadTextSafe(file string, maxBytes int64) (text string, found bool, err error) {
	f, err := openNoFollow(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	if !st.Mode().IsRegular() {
		return "", false, unsafePath("%s is not a regular file", file)
	}
	if st.Size() > maxBytes {
		return "", false, unsafePath("%s exceeds %d bytes", file, maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return "", false, err
	}
	if int64(len(data)) > maxBytes {
		return "", false, unsafePath("%s exceeds %d bytes", file, maxBytes)
	}
	return string(data), true, nil
}

// EnsurePrivateDir creates a directory tree with 0700 for the directories it creates.
func EnsurePrivateDir(dir string) error { return os.MkdirAll(dir, 0o700) }

// WriteFileAtomic writes through a random temp file created exclusively (0600) next to the
// target, fsyncs and renames it. It refuses to replace a symlink.
func WriteFileAtomic(file string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(file)
	if err := EnsurePrivateDir(dir); err != nil {
		return err
	}
	if st, err := os.Lstat(file); err == nil && st.Mode()&fs.ModeSymlink != 0 {
		return unsafePath("%s is a symlink", file)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "."+filepath.Base(file)+"."+hex.EncodeToString(rnd[:])+".tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
