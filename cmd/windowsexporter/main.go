// cmd/windowsexporter/main.go – unified exporter with file-event support
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gysosin/Logs_exporter/internal/collectors"
	"github.com/kardianos/service"
	"github.com/nats-io/nats.go"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ─────────────────────────────────── configuration ──────────────────────────
type Config struct {
	Port          string   `json:"port"`
	SystemName    string   `json:"system_name"`
	NatsURL       string   `json:"nats_url"`
	Mode          string   `json:"mode"`               // push | scrape
	NetIfaces     []string `json:"netflow_interfaces"` // optional
	WatchDir      string   `json:"watch_dir"`          // directory to audit
	WatchWindowMs int      `json:"watch_window_ms"`    // burst window (ms)
}

var config Config

// ─────────────────────────────────── logging ────────────────────────────────
func initLogging() {
	log.SetOutput(&lumberjack.Logger{
		Filename:   "logs_exporter_debug.log",
		MaxSize:    10,
		MaxAge:     7,
		MaxBackups: 1,
		Compress:   true,
	})
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Logging initialized")
}

func logWarning(f string, v ...interface{}) { log.Printf("[WARN] "+f, v...) }
func logInfo(f string, v ...interface{})    { log.Printf("[INFO] "+f, v...) }
func logError(f string, v ...interface{})   { log.Printf("[ERR ] "+f, v...) }

// ─────────────────────────────────── config loader ──────────────────────────
func loadConfig(file string) error {
	if !filepath.IsAbs(file) {
		if exe, err := os.Executable(); err == nil {
			file = filepath.Join(filepath.Dir(exe), file)
		}
	}
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &config)
}

// ─────────────────────────────────── spool helpers ──────────────────────────
const (
	spoolBase    = "spool"
	maxSpoolSize = 10 * 1024 * 1024 * 1024 // 10 GiB
	maxSpoolAge  = 24 * time.Hour
)

type fileEntry struct {
	path    string
	modTime time.Time
	size    int64
}

func ensureDir(p string) { _ = os.MkdirAll(p, 0o700) }

func subjectDir(subj string) string {
	safe := filepath.Base(subj)
	dir := filepath.Join(spoolBase, safe)
	ensureDir(dir)
	return dir
}

func spoolFileName(subj string) string {
	return filepath.Join(subjectDir(subj), fmt.Sprintf("%d.json", time.Now().UnixNano()))
}

func writeToSpool(subj string, data []byte) {
	if err := ioutil.WriteFile(spoolFileName(subj), data, 0o600); err != nil {
		logError("spool write %s: %v", subj, err)
	} else {
		logWarning("spooled offline → %s", subj)
	}
}

func flushSpool(js nats.JetStreamContext, subj string) {
	dir := subjectDir(subj)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		payload, err := ioutil.ReadFile(full)
		if err != nil {
			continue
		}
		if _, err := js.Publish(subj, payload); err == nil {
			_ = os.Remove(full)
		}
	}
}

func cleanupSpoolGlobally() {
	var files []fileEntry
	var total int64
	now := time.Now()

	_ = filepath.WalkDir(spoolBase, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, _ := d.Info()
		if now.Sub(info.ModTime()) > maxSpoolAge {
			_ = os.Remove(p)
			return nil
		}
		files = append(files, fileEntry{p, info.ModTime(), info.Size()})
		total += info.Size()
		return nil
	})

	if total <= maxSpoolSize {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	for _, f := range files {
		if total <= maxSpoolSize {
			break
		}
		_ = os.Remove(f.path)
		total -= f.size
	}
}

// ───────────────────────────── service implementation ───────────────────────
type program struct {
	Port         string
	Mode         string
	NatsURL      string
	PushInterval time.Duration
}

func (p *program) Start(s service.Service) error {
	logInfo("service start (mode=%s)", p.Mode)
	go collectors.CaptureNetFlowFromAll(config.NetIfaces)
	collectors.StartFileWatcher(config.WatchDir, config.WatchWindowMs)
	go p.run()
	if p.Mode == "push" {
		go p.pushLoop()
	}
	return nil
}
func (p *program) Stop(service.Service) error { logInfo("service stop"); return nil }

func (p *program) run() {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(collectors.GenerateMetrics()))
	})
	mux.HandleFunc("/netflow", func(w http.ResponseWriter, _ *http.Request) {
		flows := collectors.GetNetFlowEntries()
		sys := effectiveSystemName()
		for i := range flows {
			flows[i].SystemName = sys
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(flows)
	})
	mux.HandleFunc("/fileevents", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(collectors.GetFileEvents())
	})
	addr := ":" + p.Port
	logInfo("HTTP listening on %s", addr)
	_ = http.ListenAndServe(addr, mux)
}

// ───────────────────────── push loop (metrics/netflow/fileevents) ───────────
func (p *program) pushLoop() {
	sys := effectiveSystemName()
	metricSub := "metrics." + sys
	netSub := "netflow." + sys
	fileSub := "fileevents." + sys

	ticker := time.NewTicker(p.PushInterval)
	defer ticker.Stop()

	for {
		cleanupSpoolGlobally()

		mp := map[string]string{"system_name": sys, "metrics": collectors.GenerateMetrics()}
		metricPayload, _ := json.Marshal(mp)

		flows := collectors.GetNetFlowEntries()
		for i := range flows {
			flows[i].SystemName = sys
		}
		flowPayload, _ := json.Marshal(flows)
		filePayload, _ := json.Marshal(collectors.GetFileEvents())

		nc, err := nats.Connect(p.NatsURL, nats.Timeout(3*time.Second))
		if err != nil {
			writeToSpool(metricSub, metricPayload)
			writeToSpool(netSub, flowPayload)
			writeToSpool(fileSub, filePayload)
			<-ticker.C
			continue
		}
		js, err := nc.JetStream()
		if err != nil {
			nc.Drain()
			writeToSpool(metricSub, metricPayload)
			writeToSpool(netSub, flowPayload)
			writeToSpool(fileSub, filePayload)
			<-ticker.C
			continue
		}

		flushSpool(js, metricSub)
		flushSpool(js, netSub)
		flushSpool(js, fileSub)

		if _, err := js.Publish(metricSub, metricPayload); err != nil {
			writeToSpool(metricSub, metricPayload)
		}
		if _, err := js.Publish(netSub, flowPayload); err != nil {
			writeToSpool(netSub, flowPayload)
		}
		if _, err := js.Publish(fileSub, filePayload); err != nil {
			writeToSpool(fileSub, filePayload)
		} else {
			collectors.ClearFileEvents()
		}
		nc.Drain()
		<-ticker.C
	}
}

// ──────────────────────────────── helpers ───────────────────────────────────
func effectiveSystemName() string {
	if config.SystemName != "" {
		return config.SystemName
	}
	h, _ := os.Hostname()
	return h
}

func defaultWatchDir() string {
	if runtime.GOOS == "windows" {
		return `C:\Users\Public`
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "watched")
}

// ────────────────────────────────── main ────────────────────────────────────
func main() {
	initLogging()

	cfgFile := flag.String("config", "config.json", "config file")
	portFlag := flag.String("port", "", "override port")
	modeFlag := flag.String("mode", "", "push|scrape")
	pushFlag := flag.Bool("push", false, "shortcut for --mode push")
	natsFlag := flag.String("nats_url", "", "NATS URL")
	watchDirFlag := flag.String("watch_dir", "", "directory to audit")
	winMsFlag := flag.Int("watch_window_ms", 2000, "burst window ms")
	pushIntFlag := flag.String("push_interval", "5s", "push interval")
	svcOp := flag.String("service", "", "install|uninstall|start|stop|run (Windows only)")
	includeInternal := flag.Bool("include_internal", false, "include host<->host flows")
	flag.Parse()

	collectors.IncludeInternalFlows = *includeInternal
	_ = loadConfig(*cfgFile)

	if *portFlag != "" {
		config.Port = *portFlag
	}
	if *natsFlag != "" {
		config.NatsURL = *natsFlag
	}
	if *watchDirFlag != "" {
		config.WatchDir = *watchDirFlag
	}
	if *winMsFlag > 0 {
		config.WatchWindowMs = *winMsFlag
	}
	if config.Port == "" {
		config.Port = "9182"
	}
	if config.WatchDir == "" {
		config.WatchDir = defaultWatchDir()
	}
	if config.WatchWindowMs <= 0 {
		config.WatchWindowMs = 2000
	}

	mode := config.Mode
	if *modeFlag != "" {
		mode = *modeFlag
	} else if *pushFlag {
		mode = "push"
	}
	if mode == "" {
		mode = "scrape"
	}

	interval, err := time.ParseDuration(*pushIntFlag)
	if err != nil {
		interval = 5 * time.Second
	}
	logInfo("cfg: port=%s mode=%s nats=%s watch=%s win=%dms interval=%v",
		config.Port, mode, config.NatsURL, config.WatchDir, config.WatchWindowMs, interval)

	svcCfg := &service.Config{
		Name:        "LogsExporterService",
		DisplayName: "Logs Exporter Service",
		Description: "Exports system metrics, NetFlow and FileEvents",
	}

	prg := &program{Port: config.Port, Mode: mode, NatsURL: config.NatsURL, PushInterval: interval}
	s, _ := service.New(prg, svcCfg)

	// Windows service control
	if runtime.GOOS == "windows" && *svcOp != "" {
		switch *svcOp {
		case "install":
			_ = s.Install()
		case "uninstall":
			_ = s.Uninstall()
		case "start":
			_ = s.Start()
		case "stop":
			_ = s.Stop()
		case "run":
			_ = s.Run()
		default:
			logError("unknown --service op: %s", *svcOp)
		}
		return
	}

	if err := s.Run(); err != nil {
		logError("service run: %v", err)
	}
}
