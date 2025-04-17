# ⚡ Logs Exporter

Cross-platform system + network **monitoring agent** written in Go — lightweight, fast, Prometheus-ready, and NATS-integrated. Ideal for Windows, with partial Linux/macOS support.

---

## 🚀 Features

- 🌐 Prometheus `/metrics` endpoint
- 📤 Push mode with **NATS JetStream** support
- 📡 Real-time **NetFlow capture**
- 💻 System metrics:
  - CPU (total + per-process)
  - Memory (total + per-process)
  - Disk & Volume usage
  - Network stats + TCP/UDP
- 🧠 Process count & logical processors
- 🪟 Windows-only metrics:
  - Page file usage
  - Running services
  - Event logs (counts)
  - Thermal sensors
- 🛠 CLI & `config.json` configuration
- 🔁 Runs as a **Windows service**
- 📦 Inno Setup for easy installation

---

## ⚙️ Configuration

Edit `config.json`:

```json
{
  "port": "9182",
  "system_name": "agent-A",
  "nats_url": "nats://127.0.0.1:4222",
  "mode": "push",
  "netflow_interfaces": []
}
```

Or override via CLI:

```bash
netprobe_agent.exe --port 9183 --mode scrape --nats_url nats://localhost:4222
```

---

## 🛠 Build from Source

> Requires **Go 1.18+**

```bash
git clone https://github.com/yourname/netprobe-agent.git
cd netprobe-agent
go build -o netprobe_agent ./cmd/windowsexporter

# Windows
GOOS=windows GOARCH=amd64 go build -o netprobe_agent.exe ./cmd/windowsexporter

# Linux
GOOS=linux GOARCH=amd64 go build -o netprobe_agent ./cmd/windowsexporter

# macOS
GOOS=darwin GOARCH=amd64 go build -o netprobe_agent ./cmd/windowsexporter
```

---

## ▶️ Running the Agent

### Direct run

```bash
netprobe_agent.exe --config config.json
```

or with flags:

```bash
netprobe_agent.exe --port 9183 --push
```

### As a Windows Service

```bash
netprobe_agent.exe --service install
netprobe_agent.exe --service start
```

Other service actions:

```bash
--service stop
--service uninstall
--service run
```

---

## 📈 Metrics Output

Scrape metrics via:

```
http://localhost:9182/metrics
```

Example metrics include:

- `logs_exporter_cpu_usage_percent`
- `logs_exporter_process_cpu_percent`
- `logs_exporter_memory_bytes`
- `logs_exporter_process_memory_bytes`
- `logs_exporter_disk_bytes`
- `logs_exporter_volume_bytes`
- `logs_exporter_network_bytes_per_sec`
- `logs_exporter_tcp_connections_established`
- `logs_exporter_service_state`
- `logs_exporter_event_log_count`
- `logs_exporter_system_info`
- `logs_exporter_pagefile_usage_percent`
- `logs_exporter_thermalzone_celsius`

---

## 🌐 NetFlow Collection

Also exposes real-time flow data:

```
http://localhost:9182/netflow
```

Each entry includes:

- Source/destination IP & port
- Direction (inbound/outbound)
- Protocol (TCP/UDP)
- Byte and packet count
- Start/end timestamps

In **push mode**, NetFlow data is also sent to `netflow.hostname` via NATS JetStream.

---

## 📤 Push Mode (Optional)

Enable with:

```bash
--push
```

Pushes both metrics and NetFlow to NATS:

- `metrics.hostname`
- `netflow.hostname`

Set the interval using:

```bash
--push_interval 5s
```

---

## 📦 Windows Installer (Inno Setup)

Generate a `.exe` installer:

1. Install [Inno Setup](https://jrsoftware.org/isinfo.php)
2. Run:

```bash
ISCC setup.iss
```

The installer will:

- Install the binary
- Configure the service
- Drop the config file
- Start the service

---

## 🧪 Supported Platforms

| Platform      | Status  | Notes                     |
| ------------- | ------- | ------------------------- |
| Windows 10/11 | ✅ Full | Service & installer ready |
| Linux (any)   | ✅ Full | Systemd or binary mode    |
| macOS         | ✅ Full | Use launchd or manual run |
