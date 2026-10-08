//go:build windows

package security

import (
	"io/fs"
	"os"
)

// Windows has no O_NOFOLLOW: check with Lstat first, as the TypeScript version does.
func openNoFollow(file string) (*os.File, error) {
	st, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		return nil, unsafePath("%s is a symlink", file)
	}
	return os.Open(file)
}

// LooseConfigPermissions is a no-op on Windows (ACLs, not mode bits).
func LooseConfigPermissions(string) string { return "" }
