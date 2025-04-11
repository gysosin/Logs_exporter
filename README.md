# 📊 Logs Exporter

Cross-platform system/application **metrics exporter** written in Go — lightweight, fast, and Prometheus-ready. Ideal for Windows systems, with partial Linux/macOS support.

---

## 🚀 Features

- 🌐 `/metrics` endpoint (Prometheus format)
- 💻 System metrics:
  - CPU (total + per-process)
  - Memory (total + per-process)
  - Disk & Volume usage
  - Network stats + TCP/UDP
- 🧠 Process count & logical processors
- 🪟 Windows-only:
  - Page file usage
  - Running services
  - Event logs
  - Thermal sensors
- 🔁 Runs as background Windows service
- ⚙️ CLI & `config.json` support
- 📦 Inno Setup for Windows installer

---

## ⚙️ Configuration

Edit `config.json` to set the port:

```json
{
  "port": "9182"
}
```

You can also override via CLI:

```bash
logs_exporter.exe --port 9183
```

---

## 🛠 Build from Source

> Requires **Go 1.18+**

```bash
git clone https://github.com/yourname/Logs_exporter.git
cd Logs_exporter
go build -o logs_exporter ./cmd/windowsexporter


# Windows
GOOS=windows GOARCH=amd64 go build -o logs_exporter.exe ./cmd/windowsexporter

# Linux
GOOS=linux GOARCH=amd64 go build -o logs_exporter ./cmd/windowsexporter

# macOS
GOOS=darwin GOARCH=amd64 go build -o logs_exporter ./cmd/windowsexporter

```

---

## 🚀 Running

### ▶️ Run directly

```bash
logs_exporter.exe --config config.json
```

or

```bash
logs_exporter.exe --port 9183
```

### 🪟 Run as Windows Service

```bash
logs_exporter.exe --service install
logs_exporter.exe --service start
```

Other valid actions:

```bash
--service stop
--service uninstall
--service run
```

---

## 📈 Metrics Output

Visit in browser or Prometheus scrape:

```
http://localhost:9182/metrics
```

Includes:

- `windows_cpu_usage_percent`
- `windows_process_cpu_percent`
- `windows_memory_bytes`
- `windows_disk_bytes`
- `windows_volume_bytes`
- `windows_network_bytes_per_sec`
- `windows_tcp_connections_established`
- `windows_service_state`
- `windows_event_log_count`
- And many more...

---

## 📦 Windows Installer (Inno Setup)

To create a `.exe` installer:

1. Download and install [Inno Setup](https://jrsoftware.org/isinfo.php)
2. Run this from project root:

```bash
ISCC setup.iss
```

This generates a full installer that:

- Installs the binary
- Registers and starts the Windows service
- Sets up config file

---

## 🧪 Supported Platforms

| Platform      | Status  | Notes                     |
| ------------- | ------- | ------------------------- |
| Windows 10/11 | ✅ Full | Service & installer ready |
| Linux (any)   | ✅ Full | Systemd or binary mode    |
| macOS         | ✅ Full | Use launchd or manual run |
