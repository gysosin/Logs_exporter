package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"log"

	"github.com/gysosin/Logs_exporter/internal/collectors"
	"github.com/kardianos/service"
	"github.com/nats-io/nats.go"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds runtime configuration read from JSON.
type Config struct {
	Port       string `json:"port"`
	SystemName string `json:"system_name"`
	NatsURL    string `json:"nats_url"`
	Mode       string `json:"mode"` // "push" or "scrape"
}

var config Config

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

func logWarning(format string, v ...any) {
	log.Printf("[WARNING] "+format, v...)
}

func logError(format string, v ...any) {
	log.Printf("[ERROR] "+format, v...)
}

func loadConfig(filename string) error {
	if !filepath.IsAbs(filename) {
		exePath, err := os.Executable()
		if err == nil {
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

type program struct {
	Port         string
	Mode         string
	NatsURL      string
	PushInterval time.Duration
}

func (p *program) Start(s service.Service) error {
	logWarning("Service starting with mode=%s", p.Mode)
	if p.Mode == "push" {
		go pushMetrics(p.NatsURL, p.PushInterval)
	} else {
		go p.run()
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
	if err := http.ListenAndServe(addr, nil); err != nil {
		logError("HTTP server failed: %v", err)
	}
}

func (p *program) Stop(s service.Service) error {
	logWarning("Service stopping")
	os.Exit(0)
	return nil
}

func pushMetrics(natsURL string, interval time.Duration) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logError("Failed to connect to NATS: %v", err)
		return
	}
	defer nc.Drain()

	js, err := nc.JetStream()
	if err != nil {
		logError("Failed to get JetStream context: %v", err)
		return
	}

	subject := "metrics"

	hostname, err := os.Hostname()
	if err != nil {
		logError("Unable to get hostname: %v", err)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		metrics := collectors.GenerateMetrics()
		payload := map[string]string{
			"system_name": hostname,
			"metrics":     metrics,
		}
		msgPayload, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			logError("Failed to marshal metrics payload: %v", err)
			continue
		}
		_, err = js.Publish(subject, msgPayload)
		if err != nil {
			logError("Failed to publish metrics: %v", err)
		}
	}
}

func main() {
	initLogging()

	svcConfig := &service.Config{
		Name:        "LogsExporterService",
		DisplayName: "Logs Exporter Service",
		Description: "Exports system metrics in Prometheus format (either push mode or scrape).",
	}

	configFile := flag.String("config", "config.json", "Path to JSON config file")
	svcFlag := flag.String("service", "", "Install/uninstall/start/stop/run the Windows service (example: --service=install)")
	portFlag := flag.String("port", "", "Override port from config.json (e.g. 9182)")
	pushFlag := flag.Bool("push", false, "Enable push mode (publish to NATS JetStream)")
	modeFlag := flag.String("mode", "", "Mode (push or scrape)")
	natsURLFlag := flag.String("nats_url", "", "NATS server URL")
	pushIntervalFlag := flag.String("push_interval", "1s", "How often to push metrics, e.g. 500ms, 2s")
	flag.Parse()

	wd, err := os.Getwd()
	if err == nil {
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
	if *modeFlag != "" {
		mode = *modeFlag
	} else if *pushFlag {
		mode = "push"
	} else if config.Mode != "" {
		mode = config.Mode
	} else {
		mode = "scrape"
	}

	interval, err := time.ParseDuration(*pushIntervalFlag)
	if err != nil {
		logWarning("Invalid push_interval=%s. Defaulting to 1s", *pushIntervalFlag)
		interval = time.Second
	}

	logWarning("Effective Config: Port=%s, NatsURL=%s, Mode=%s, PushInterval=%v", config.Port, config.NatsURL, mode, interval)

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

	// only handle service flag on Windows
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
