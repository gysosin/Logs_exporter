//go:build linux
// +build linux

package collectors

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// StartFileWatcher starts a fanotify watcher in its own goroutine.
// It is NOP-safe if called multiple times.
func StartFileWatcher(dir string, windowMs int) {
	go runLinuxWatcher(dir, windowMs) // detached
}

// ---------------------------------------------------------------------------
// fanotify core (adapted from your previous linux_watcher.go) ---------------
// ---------------------------------------------------------------------------

func runLinuxWatcher(dir string, windowMs int) {
	// ensure absolute path
	dir, _ = filepath.Abs(dir)

	fd, err := unix.FanotifyInit(unix.FAN_CLASS_NOTIF|unix.FAN_NONBLOCK|unix.FAN_CLOEXEC, unix.O_RDONLY)
	if err != nil {
		log.Printf("fanotify init failed: %v", err)
		return
	}
	defer unix.Close(fd)

	mask := uint64(
		unix.FAN_ACCESS |
			unix.FAN_MODIFY |
			unix.FAN_CLOSE_WRITE |
			unix.FAN_CLOSE_NOWRITE |
			unix.FAN_OPEN |
			unix.FAN_EVENT_ON_CHILD,
	)
	if err := unix.FanotifyMark(fd, unix.FAN_MARK_ADD, mask, unix.AT_FDCWD, dir); err != nil {
		log.Printf("fanotify mark failed: %v", err)
		return
	}

	pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	buf := make([]byte, 4096)

	for {
		_, err := unix.Poll(pfds, -1)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			log.Printf("poll error: %v", err)
			return
		}

		n, err := unix.Read(fd, buf)
		if err != nil {
			if err == unix.EAGAIN {
				continue
			}
			log.Printf("fanotify read failed: %v", err)
			return
		}

		for off := 0; off < n; {
			meta := (*unix.FanotifyEventMetadata)(unsafe.Pointer(&buf[off]))
			if meta.Event_len < uint32(unsafe.Sizeof(*meta)) {
				break
			}

			if meta.Fd >= 0 {
				link := fmt.Sprintf("/proc/self/fd/%d", meta.Fd)
				resolved, _ := os.Readlink(link)
				unix.Close(int(meta.Fd))

				if resolved == "" || resolved == dir {
					off += int(meta.Event_len)
					continue
				}

				uid := getUidFromPid(int(meta.Pid))
				username := "unknown"
				if u, err := user.LookupId(strconv.Itoa(uid)); err == nil {
					username = u.Username
				}

				AddFileEvent(FileEvent{
					Path:      resolved,
					Event:     decodeMask(meta.Mask),
					PID:       int(meta.Pid),
					UID:       uid,
					User:      username,
					Timestamp: time.Now(),
				})
			}
			off += int(meta.Event_len)
		}
	}
}

func getUidFromPid(pid int) int {
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if uid, _ := strconv.Atoi(fields[1]); uid >= 0 {
					return uid
				}
			}
		}
	}
	return -1
}

func decodeMask(mask uint64) string {
	switch {
	case mask&unix.FAN_MODIFY != 0:
		return "WRITE"
	case mask&unix.FAN_CLOSE_WRITE != 0:
		return "CLOSE_WRITE"
	case mask&unix.FAN_CLOSE_NOWRITE != 0:
		return "CLOSE_NOWRITE"
	case mask&unix.FAN_ACCESS != 0:
		return "ACCESS"
	case mask&unix.FAN_OPEN != 0:
		return "OPEN"
	default:
		return "UNKNOWN"
	}
}
