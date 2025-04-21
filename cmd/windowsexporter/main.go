// main.go
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

type Config struct {
	Port       string   `json:"port"`
	SystemName string   `json:"system_name"`
	NatsURL    string   `json:"nats_url"`
	Mode       string   `json:"mode"`               // "push" or "scrape"
	NetIfaces  []string `json:"netflow_interfaces"` // optional
}

var config Config

// -------- logging helpers --------------------------------------------------

func initLogging() {
	log.SetOutput(&lumberjack.Logger{
		Filename:   "logs_exporter_debug.log",
		MaxSize:    10,
		MaxAge:     7,
		MaxBackups: 1,
		Compress:   true,
	})
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Logging initialized: WARNING and ERROR only")
}

func logWarning(format string, v ...interface{}) { log.Printf("[WARNING] "+format, v...) }
func logInfo(format string, v ...interface{})    { log.Printf("[INFO] "+format, v...) }
func logError(format string, v ...interface{})   { log.Printf("[ERROR] "+format, v...) }

// -------- config ------------------------------------------------------------

func loadConfig(filename string) error {
	if !filepath.IsAbs(filename) {
		if exePath, err := os.Executable(); err == nil {
			filename = filepath.Join(filepath.Dir(exePath), filename)
		} else {
			logError("Error determining executable path: %v", err)
		}
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
}

// -------- spool helpers -----------------------------------------------------

const (
	spoolBase    = "spool"
	maxSpoolSize = 10 * 1024 * 1024 * 1024 // 10 GiB
	maxSpoolAge  = 24 * time.Hour          // 24h retention
)

// ensureDir makes sure path exists.
func ensureDir(path string) {
	_ = os.MkdirAll(path, 0o700)
}

// subjectDir returns spoolBase/<subject> path.
func subjectDir(subject string) string {
	safe := filepath.Base(subject)
	dir := filepath.Join(spoolBase, safe)
	ensureDir(dir)
	return dir
}

// spoolFileName returns a new filename under the subject subdir.
func spoolFileName(subject string) string {
	now := time.Now().UnixNano()
	return filepath.Join(subjectDir(subject), fmt.Sprintf("%d.json", now))
}

// writeToSpool persists a payload to disk under its subject dir.
func writeToSpool(subject string, data []byte) {
	path := spoolFileName(subject)
	if err := ioutil.WriteFile(path, data, 0o600); err != nil {
		logError("Failed to spool %s: %v", subject, err)
	} else {
		logWarning("Spooling message to %s (NATS unreachable)", path)
	}
}

// flushSpool publishes all files in subject’s spool subdir.
func flushSpool(js nats.JetStreamContext, subject string) {
	dir := subjectDir(subject)
	entries, err := os.ReadDir(dir)
	if err != nil {
		logError("ReadDir %s: %v", dir, err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		payload, err := ioutil.ReadFile(full)
		if err != nil {
			logError("Read spool %s: %v", full, err)
			continue
		}
		if _, err := js.Publish(subject, payload); err != nil {
			logWarning("Re‑publish of %s failed: %v", full, err)
			continue
		}
		if err := os.Remove(full); err != nil {
			logError("Delete spool %s: %v", full, err)
		} else {
			logInfo("Re‑published and removed %s", full)
		}
	}
}

// cleanupSpoolGlobally enforces age and size limits across all subjects.
func cleanupSpoolGlobally() {
	type fileEntry struct {
		path    string
		modTime time.Time
		size    int64
	}
	var files []fileEntry
	var totalSize int64

	now := time.Now()
	// Walk all subject subdirs
	filepath.WalkDir(spoolBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		age := now.Sub(info.ModTime())
		// delete if too old
		if age > maxSpoolAge {
			if rmErr := os.Remove(path); rmErr != nil {
				logError("cleanup delete old %s: %v", path, rmErr)
			} else {
				logInfo("Removed old spool file %s", path)
			}
			return nil
		}
		// keep for size-based eviction
		files = append(files, fileEntry{path: path, modTime: info.ModTime(), size: info.Size()})
		totalSize += info.Size()
		return nil
	})

	// if over size, delete oldest first
	if totalSize <= maxSpoolSize {
		return
	}
	// sort by oldest
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, f := range files {
		if totalSize <= maxSpoolSize {
			break
		}
		if err := os.Remove(f.path); err != nil {
			logError("cleanup delete for size %s: %v", f.path, err)
			continue
		}
		totalSize -= f.size
		logInfo("Removed spool file for size limit: %s", f.path)
	}
}

// -------- service implementation -------------------------------------------

type program struct {
	Port         string
	Mode         string
	NatsURL      string
	PushInterval time.Duration
}

func (p *program) Start(s service.Service) error {
	logWarning("Service starting with mode=%s", p.Mode)
	go collectors.CaptureNetFlowFromAll(config.NetIfaces)
	go p.run()
	if p.Mode == "push" {
		go pushMetrics(p.NatsURL, p.PushInterval)
	}
	return nil
}

func (p *program) run() {
	addr := ":" + p.Port
	logWarning("Starting HTTP server on %s...", addr)

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := collectors.GenerateMetrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics))
	})

	http.HandleFunc("/netflow", func(w http.ResponseWriter, r *http.Request) {
		entries := collectors.GetNetFlowEntries()
		systemName := config.SystemName
		if systemName == "" {
			if hn, err := os.Hostname(); err == nil {
				systemName = hn
			}
		}
		for i := range entries {
			entries[i].SystemName = systemName
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		logError("HTTP server failed: %v", err)
	}
}

func (p *program) Stop(s service.Service) error {
	logWarning("Service stopping")
	return nil
}

// -------- resilient push logic ---------------------------------------------

func pushMetrics(natsURL string, interval time.Duration) {
	defer logInfo("pushMetrics stopped")

	hostname, _ := os.Hostname()
	systemName := config.SystemName
	if systemName == "" {
		systemName = hostname
	}

	metricSubject := "metrics." + systemName
	netflowSubject := "netflow." + systemName

	logInfo("Will publish to %q and %q every %v", metricSubject, netflowSubject, interval)

	publish := func(js nats.JetStreamContext, subj string, data []byte) error {
		_, err := js.Publish(subj, data)
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// clean up spool before anything else
		cleanupSpoolGlobally()

		// gather payloads
		metrics := collectors.GenerateMetrics()
		mp := map[string]string{"system_name": systemName, "metrics": metrics}
		metricPayload, _ := json.MarshalIndent(mp, "", "  ")

		netflows := collectors.GetNetFlowEntries()
		for i := range netflows {
			netflows[i].SystemName = systemName
		}
		netflowPayload, _ := json.MarshalIndent(netflows, "", "  ")

		// try NATS
		nc, err := nats.Connect(natsURL, nats.Timeout(3*time.Second))
		if err != nil {
			logWarning("NATS connect failed: %v", err)
			writeToSpool(metricSubject, metricPayload)
			writeToSpool(netflowSubject, netflowPayload)
			<-ticker.C
			continue
		}

		js, err := nc.JetStream()
		if err != nil {
			logWarning("JetStream init failed: %v", err)
			writeToSpool(metricSubject, metricPayload)
			writeToSpool(netflowSubject, netflowPayload)
			nc.Drain()
			<-ticker.C
			continue
		}

		// flush old spools per subject
		flushSpool(js, metricSubject)
		flushSpool(js, netflowSubject)

		// publish new
		if err := publish(js, metricSubject, metricPayload); err != nil {
			logWarning("Publish metrics failed: %v", err)
			writeToSpool(metricSubject, metricPayload)
		}
		if err := publish(js, netflowSubject, netflowPayload); err != nil {
			logWarning("Publish netflow failed: %v", err)
			writeToSpool(netflowSubject, netflowPayload)
		}

		nc.Drain()
		<-ticker.C
	}
}

// -------- entry‑point -------------------------------------------------------

func main() {
	initLogging()

	svcConfig := &service.Config{
		Name:        "LogsExporterService",
		DisplayName: "Logs Exporter Service",
		Description: "Exports system metrics and NetFlow data (push/scrape mode)",
	}

	configFile := flag.String("config", "config.json", "Path to JSON config file")
	svcFlag := flag.String("service", "", "Install/uninstall/start/stop/run the Windows service")
	portFlag := flag.String("port", "", "Override port from config.json")
	pushFlag := flag.Bool("push", false, "Enable push mode")
	modeFlag := flag.String("mode", "", "Mode (push or scrape)")
	natsURLFlag := flag.String("nats_url", "", "NATS server URL")
	pushIntervalFlag := flag.String("push_interval", "5s", "Interval for push mode")

	flag.Parse()

	if wd, err := os.Getwd(); err == nil {
		logWarning("Working Directory: %s", wd)
	}

	if err := loadConfig(*configFile); err != nil {
		logWarning("Could not read config file %s. Using defaults: %v", *configFile, err)
		config.Port = "9182"
		config.SystemName = ""
		config.NatsURL = "nats://127.0.0.1:4222"
		config.Mode = "scrape"
	}

	if *portFlag != "" {
		config.Port = *portFlag
	}
	if *natsURLFlag != "" {
		config.NatsURL = *natsURLFlag
	}

	var mode string
	switch {
	case *modeFlag != "":
		mode = *modeFlag
	case *pushFlag:
		mode = "push"
	case config.Mode != "":
		mode = config.Mode
	default:
		mode = "scrape"
	}

	interval, err := time.ParseDuration(*pushIntervalFlag)
	if err != nil {
		logWarning("Invalid push_interval=%s. Defaulting to 5s", *pushIntervalFlag)
		interval = 5 * time.Second
	}

	logWarning("Effective Config: Port=%s, NatsURL=%s, Mode=%s, PushInterval=%v",
		config.Port, config.NatsURL, mode, interval)

	prg := &program{
		Port:         config.Port,
		Mode:         mode,
		NatsURL:      config.NatsURL,
		PushInterval: interval,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		logError("Cannot start service: %v", err)
	}

	if runtime.GOOS == "windows" && *svcFlag != "" {
		if err := service.Control(s, *svcFlag); err != nil {
			logError("Valid service actions: install, uninstall, start, stop, run")
			logError("%v", err)
			return
		}
		logWarning("Service action '%s' executed successfully.", *svcFlag)
		return
	} else if runtime.GOOS != "windows" && *svcFlag != "" {
		logWarning("--service is only supported on Windows. Ignoring.")
	}

	if err := s.Run(); err != nil {
		logError("%v", err)
	}
}
