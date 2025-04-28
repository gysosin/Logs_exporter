//go:build windows
// +build windows

package collectors

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StartFileWatcher launches FolderMonitor.exe and pipes stdout.
func StartFileWatcher(dir string, windowMs int) {
	go runWindowsWatcher(dir, windowMs)
}

var lineRe = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{2} [\d:]+)] \[ETW\] (\w+) .* PID (\d+).*\): (.*)$`)

func runWindowsWatcher(dir string, windowMs int) {
	exePath := filepath.Join(filepath.Dir(os.Args[0]), "FolderMonitor.exe")
	if _, err := os.Stat(exePath); err != nil {
		// fallback to no-op if exe missing
		return
	}

	cmd := exec.Command(exePath, dir, "audit.log", strconv.Itoa(windowMs))
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		txt := scanner.Text()
		m := lineRe.FindStringSubmatch(txt)
		if len(m) != 5 {
			continue
		}
		ts, _ := time.Parse("2006-01-02 15:04:05", m[1])
		evt := strings.ToUpper(m[2])
		pid, _ := strconv.Atoi(m[3])
		path := m[4]

		AddFileEvent(FileEvent{
			Path:      path,
			Event:     evt,
			PID:       pid,
			User:      "", // FolderMonitor already resolves user in message, not needed here
			Timestamp: ts,
		})
	}
	_ = cmd.Wait()
}
