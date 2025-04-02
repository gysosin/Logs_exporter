package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gysosin/Logs_exporter/internal/collectors"
	"github.com/kardianos/service"
)

// Config holds runtime configuration.
type Config struct {
	Port string `json:"port"`
}

var config Config

// loadConfig reads a JSON configuration file into the config struct.
func loadConfig(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
}

// program implements service.Interface.
type program struct{}

func (p *program) Start(s service.Service) error {
	// Run the service asynchronously.
	go p.run()
	return nil
}

func (p *program) run() {
	addr := ":" + config.Port
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := collectors.GenerateMetrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics))
	})
	log.Printf("Starting HTTP server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func (p *program) Stop(s service.Service) error {
	log.Println("Service stopping")
	// Signal termination if needed.
	os.Exit(0)
	return nil
}

func main() {
	// Service configuration.
	svcConfig := &service.Config{
		Name:        "LogsExporterService",
		DisplayName: "Logs Exporter Service",
		Description: "Exports system metrics in Prometheus format as a background service.",
	}

	// Command-line flags.
	configFile := flag.String("config", "config.json", "Path to configuration file")
	svcFlag := flag.String("service", "", "Control the system service. Valid actions: install, uninstall, start, stop, run")
	flag.Parse()

	// Load configuration from file.
	err := loadConfig(*configFile)
	if err != nil {
		log.Printf("Failed to load config file %s: %v. Using default port 9182.", *configFile, err)
		config.Port = "9182"
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	// If a service control action was provided, execute it.
	if *svcFlag != "" {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: install, uninstall, start, stop, run")
			log.Fatal(err)
		}
		log.Printf("Service action '%s' executed.", *svcFlag)
		return
	}

	// Run the service.
	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
