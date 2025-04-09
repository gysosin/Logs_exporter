package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gysosin/Logs_exporter/internal/collectors"
	"github.com/kardianos/service"
	"github.com/nats-io/nats.go"
)

// Config holds runtime configuration read from JSON (optional).
type Config struct {
	Port       string `json:"port"`
	SystemName string `json:"system_name"`
	NatsURL    string `json:"nats_url"`
}

// Global config
var config Config

// loadConfig reads a JSON configuration file into the config struct.
func loadConfig(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
}

// program implements service.Interface for running as a Windows service.
type program struct {
	Port string
}

// Start is called when the service starts.
func (p *program) Start(s service.Service) error {
	// Run the service asynchronously.
	go p.run()
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

	// Use the fixed subject "metrics".
	subject := "metrics"
	log.Printf("Starting push to NATS JetStream at subject=%s every=%v", subject, interval)

	// If no system name is specified in config, try to use the hostname.
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
		// Wrap the metrics payload in a JSON object that includes the system name.
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
	// Service config for the Windows service.
	svcConfig := &service.Config{
		Name:        "LogsExporterService",
		DisplayName: "Logs Exporter Service",
		Description: "Exports system metrics in Prometheus format (either push mode or scrape).",
	}

	// Flags.
	configFile := flag.String("config", "config.json", "Path to JSON config file")
	svcFlag := flag.String("service", "", "Install/uninstall/start/stop/run the Windows service (example: --service=install)")
	// Basic port override.
	portFlag := flag.String("port", "", "Override port from config.json (e.g. 9182)")
	// NATS push mode flag.
	pushFlag := flag.Bool("push", false, "Enable push mode (publish to NATS JetStream)")
	// nats_url flag (overriding config value if needed).
	natsURLFlag := flag.String("nats_url", "", "NATS server URL")
	pushIntervalFlag := flag.String("push_interval", "1s", "How often to push metrics, e.g. 500ms, 2s")

	flag.Parse()

	// Load the config.
	if err := loadConfig(*configFile); err != nil {
		log.Printf("Could not read config file %s. Using defaults: %v", *configFile, err)
		config.Port = "9182"
		// Do not force a default system name here
		config.SystemName = ""
		config.NatsURL = "nats://127.0.0.1:4222"
	}

	// Override config values from flags if provided.
	if *portFlag != "" {
		config.Port = *portFlag
	}
	if *natsURLFlag != "" {
		config.NatsURL = *natsURLFlag
	}

	// Parse the push interval.
	interval, err := time.ParseDuration(*pushIntervalFlag)
	if err != nil {
		log.Printf("Invalid push_interval=%s. Defaulting to 1s", *pushIntervalFlag)
		interval = time.Second
	}

	// If push mode is enabled, run pushMetrics.
	if *pushFlag {
		pushMetrics(config.NatsURL, interval)
		return
	}

	// If not push mode, run the HTTP endpoint or service.
	prg := &program{Port: config.Port}
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

	// Run the HTTP endpoint in the foreground.
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}
