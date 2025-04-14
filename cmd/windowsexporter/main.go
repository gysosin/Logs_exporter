package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

	hostname, err := os.Hostname()
	if err != nil {
		logError("Unable to get hostname: %v", err)
		return
	}

	metricSubject := "metrics." + hostname
	netflowSubject := "netflow." + hostname

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C

		// Push metrics
		metrics := collectors.GenerateMetrics()
		metricPayload := map[string]string{
			"system_name": hostname,
			"metrics":     metrics,
		}
		msgPayload, err := json.MarshalIndent(metricPayload, "", "  ")
		if err == nil {
			_, err = js.Publish(metricSubject, msgPayload)
			if err != nil {
				logError("Failed to publish metrics: %v", err)
			}
		} else {
			logError("Failed to marshal metrics payload: %v", err)
		}

		// Push NetFlow data
		netflows := collectors.GetNetFlowEntries()
		netflowPayload, err := json.MarshalIndent(netflows, "", "  ")
		if err == nil {
			_, err = js.Publish(netflowSubject, netflowPayload)
			if err != nil {
				logError("Failed to publish netflow data: %v", err)
			}
		} else {
			logError("Failed to marshal netflow payload: %v", err)
		}
	}
}

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
	pushIntervalFlag := flag.String("push_interval", "1s", "Interval for push mode")

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
