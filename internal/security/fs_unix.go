//go:build !windows

package security

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func openNoFollow(file string) (*os.File, error) {
	f, err := os.OpenFile(file, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, unsafePath("%s is a symlink", file)
	}
	return f, err
}

// LooseConfigPermissions describes a config file writable by group/others or not owned by us
// (SR-005). It returns "" when the file is fine or cannot be inspected.
func LooseConfigPermissions(file string) string {
	st, err := os.Lstat(file)
	if err != nil {
		return ""
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		return file + " is a symlink"
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != os.Getuid() {
		return fmt.Sprintf("%s is owned by uid %d", file, sys.Uid)
	}
	if perm := st.Mode().Perm(); perm&0o022 != 0 {
		return fmt.Sprintf("%s is writable by group/others (%o)", file, perm)
	}
	return ""
}
