package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gysosin/Logs_exporter/internal/collectors"
	"github.com/kardianos/service"
	"github.com/nats-io/nats.go"
)

// Config holds runtime configuration read from JSON.
type Config struct {
	Port       string `json:"port"`
	SystemName string `json:"system_name"`
	NatsURL    string `json:"nats_url"`
	Mode       string `json:"mode"` // "push" or "scrape"
}

// Global config
var config Config

// loadConfig reads a JSON configuration file into the config struct.
// If filename is a relative path, it will be resolved relative to the executable's directory.
func loadConfig(filename string) error {
	if !filepath.IsAbs(filename) {
		exePath, err := os.Executable()
		if err == nil {
			filename = filepath.Join(filepath.Dir(exePath), filename)
		} else {
			log.Printf("Error determining executable path: %v", err)
		}
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
}

// initLogging sets up log output to a file.
func initLogging() {
	// Determine the absolute path for the log file relative to the executable.
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Cannot determine executable path: %v", err)
		return
	}
	exeDir := filepath.Dir(exePath)
	logFilePath := filepath.Join(exeDir, "logs_exporter_debug.log")

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open log file %s: %v", logFilePath, err)
		return
	}
	// Redirect standard logger output to the file.
	log.SetOutput(logFile)
	log.Printf("Logging started. Log file: %s", logFilePath)
}

// program implements service.Interface for running as a Windows service.
type program struct {
	Port         string
	Mode         string // "push" or "scrape"
	NatsURL      string
	PushInterval time.Duration
}

// Start is called when the service starts.
func (p *program) Start(s service.Service) error {
	log.Printf("Service starting with mode=%s", p.Mode)
	// Choose mode: push mode will publish metrics to NATS; scrape mode serves an HTTP endpoint.
	if p.Mode == "push" {
		go pushMetrics(p.NatsURL, p.PushInterval)
	} else {
		go p.run()
	}
	return nil
}

// run sets up the /metrics endpoint and listens on p.Port.
func (p *program) run() {
	addr := ":" + p.Port
	log.Printf("Starting HTTP server on %s...", addr)
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := collectors.GenerateMetrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics))
	})
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// Stop is called when the service stops.
func (p *program) Stop(s service.Service) error {
	log.Println("Service stopping")
	os.Exit(0)
	return nil
}

// pushMetrics connects to NATS JetStream and periodically publishes metrics.
func pushMetrics(natsURL string, interval time.Duration) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to get JetStream context: %v", err)
	}

	subject := "metrics"
	log.Printf("Starting push to NATS JetStream at subject=%s every=%v", subject, interval)

	// Use hostname as system name if not specified in config.
	if config.SystemName == "" {
		hn, err := os.Hostname()
		if err != nil {
			log.Fatalf("System name not specified and unable to get hostname: %v", err)
		}
		config.SystemName = hn
		log.Printf("No system_name in config; using hostname: %s", config.SystemName)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		metrics := collectors.GenerateMetrics()
		payload := map[string]string{
			"system_name": config.SystemName,
			"metrics":     metrics,
		}
		msgPayload, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal metrics payload: %v", err)
			continue
		}
		_, err = js.Publish(subject, msgPayload)
		if err != nil {
			log.Printf("Failed to publish metrics: %v", err)
		} else {
			log.Printf("Published metrics to subject %s", subject)
		}
	}
}

func main() {
	// Initialize logging to file.
	initLogging()

	// Service configuration for the Windows service.
	svcConfig := &service.Config{
		Name:        "LogsExporterService",
		DisplayName: "Logs Exporter Service",
		Description: "Exports system metrics in Prometheus format (either push mode or scrape).",
	}

	// Command-line flags.
	configFile := flag.String("config", "config.json", "Path to JSON config file")
	svcFlag := flag.String("service", "", "Install/uninstall/start/stop/run the Windows service (example: --service=install)")
	portFlag := flag.String("port", "", "Override port from config.json (e.g. 9182)")
	pushFlag := flag.Bool("push", false, "Enable push mode (publish to NATS JetStream)")
	modeFlag := flag.String("mode", "", "Mode (push or scrape)")
	natsURLFlag := flag.String("nats_url", "", "NATS server URL")
	pushIntervalFlag := flag.String("push_interval", "1s", "How often to push metrics, e.g. 500ms, 2s")
	flag.Parse()

	// Log the current working directory for troubleshooting.
	wd, err := os.Getwd()
	if err == nil {
		log.Printf("Working Directory: %s", wd)
	}

	// Load configuration from file.
	if err := loadConfig(*configFile); err != nil {
		log.Printf("Could not read config file %s. Using defaults: %v", *configFile, err)
		config.Port = "9182"
		config.SystemName = ""
		config.NatsURL = "nats://127.0.0.1:4222"
		config.Mode = "scrape"
	}

	// Override config values from flags, if provided.
	if *portFlag != "" {
		config.Port = *portFlag
	}
	if *natsURLFlag != "" {
		config.NatsURL = *natsURLFlag
	}

	// Determine the operating mode. Priority: mode flag > push flag > config file.
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

	// Parse the push interval.
	interval, err := time.ParseDuration(*pushIntervalFlag)
	if err != nil {
		log.Printf("Invalid push_interval=%s. Defaulting to 1s", *pushIntervalFlag)
		interval = time.Second
	}

	// Log effective configuration.
	log.Printf("Effective Config: Port=%s, NatsURL=%s, Mode=%s, PushInterval=%v", config.Port, config.NatsURL, mode, interval)

	// Create a program instance with the proper configuration.
	prg := &program{
		Port:         config.Port,
		Mode:         mode,
		NatsURL:      config.NatsURL,
		PushInterval: interval,
	}

	// Create the service.
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("Cannot start service: %v", err)
	}

	// Handle service control actions.
	if *svcFlag != "" {
		if err := service.Control(s, *svcFlag); err != nil {
			log.Printf("Valid service actions: install, uninstall, start, stop, run")
			log.Fatal(err)
		}
		log.Printf("Service action '%s' executed successfully.", *svcFlag)
		return
	}

	// Run the service. The program's Start() function will branch based on mode.
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}
