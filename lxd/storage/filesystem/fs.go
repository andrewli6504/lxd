package filesystem

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/lxc/lxd/shared/logger"
)

// Filesystem magic numbers.
const (
	FilesystemSuperMagicZfs = 0x2fc12fc1
)

// StatVFS retrieves Virtual File System (VFS) info about a path.
func StatVFS(path string) (*unix.Statfs_t, error) {
	var st unix.Statfs_t

	err := unix.Statfs(path, &st)
	if err != nil {
		return nil, err
	}

	return &st, nil
}

// Detect returns the filesystem on which the passed-in path sits.
func Detect(path string) (string, error) {
	fs, err := StatVFS(path)
	if err != nil {
		return "", err
	}

	return FSTypeToName(int32(fs.Type))
}

// FSTypeToName returns the name of the given fs type.
// The fsType is from the Type field of unix.Statfs_t. We use int32 so that this function behaves the same on both
// 32bit and 64bit platforms by requiring any 64bit FS types to be overflowed before being passed in. They will
// then be compared with equally overflowed FS type constant values.
func FSTypeToName(fsType int32) (string, error) {
	// This function is needed to allow FS type constants that overflow an int32 to be overflowed without a
	// compile error on 32bit platforms. This allows us to use any 64bit constants from the unix package on
	// both 64bit and 32bit platforms without having to define the constant in its rolled over form on 32bit.
	to32 := func(fsType int64) int32 {
		return int32(fsType)
	}

	switch fsType {
	case to32(unix.BTRFS_SUPER_MAGIC): // BTRFS' constant required overflowing to an int32.
		return "btrfs", nil
	case unix.TMPFS_MAGIC:
		return "tmpfs", nil
	case unix.EXT4_SUPER_MAGIC:
		return "ext4", nil
	case unix.XFS_SUPER_MAGIC:
		return "xfs", nil
	case unix.NFS_SUPER_MAGIC:
		return "nfs", nil
	case FilesystemSuperMagicZfs:
		return "zfs", nil
	}

	logger.Debugf("Unknown backing filesystem type: 0x%x", fsType)
	return fmt.Sprintf("0x%x", fsType), nil
}

func parseMountinfo(name string) int {
	// In case someone uses symlinks we need to look for the actual
	// mountpoint.
	actualPath, err := filepath.EvalSymlinks(name)
	if err != nil {
		return -1
	}

	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return -1
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Fields(line)
		if len(tokens) < 5 {
			return -1
		}
		cleanPath := filepath.Clean(tokens[4])
		if cleanPath == actualPath {
			return 1
		}
	}

	return 0
}

// IsMountPoint returns true if path is a mount point.
func IsMountPoint(path string) bool {
	// If we find a mount entry, it is obviously a mount point.
	ret := parseMountinfo(path)
	if ret == 1 {
		return true
	}

	// Get the stat details.
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	rootStat, err := os.Lstat(path + "/..")
	if err != nil {
		return false
	}

	// If the directory has the same device as parent, then it's not a mountpoint.
	if stat.Sys().(*syscall.Stat_t).Dev == rootStat.Sys().(*syscall.Stat_t).Dev {
		return false
	}

	// Btrfs annoyingly uses a different Dev id for different subvolumes on the same mount.
	// So for btrfs, we require a matching mount entry in mountinfo.
	fs, _ := Detect(path)
	if err == nil && fs == "btrfs" {
		return false
	}

	return true
}
