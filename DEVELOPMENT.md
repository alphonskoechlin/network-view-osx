# Development Guide

## Architecture Overview

### Backend (Go)
- **mDNS Discovery**: Uses `miekg/dns` library to query mDNS services on `224.0.0.251:5353`
- **Service Streaming**: HTTP EventSource API for real-time service streaming to clients
- **Service Types Supported**:
  - `_http._tcp.local.` - Web services
  - `_https._tcp.local.` - Secure web services
  - `_ssh._tcp.local.` - SSH services
  - `_sftp._tcp.local.` - SFTP services
  - `_smb._tcp.local.` - SMB/Windows shares
  - `_afpovertcp._tcp.local.` - Apple Filing Protocol
  - `_nfs._tcp.local.` - NFS shares
  - And more...

### Frontend (Svelte)
- **Carbon Design System**: Professional UI components
- **Real-time Updates**: EventSource for streaming service updates
- **Data Table**: Responsive table displaying services
- **Search & Filter**: Client-side filtering by name, type, host, IP

## Running the Application

### Terminal 1: Backend
```bash
cd backend
go run main.go
# Server starts on http://localhost:9999
```

### Terminal 2: Frontend
```bash
cd frontend
npm install  # First time only
npm run dev
# Frontend runs on http://localhost:5173
```

Open http://localhost:5173 in your browser.

## Project Structure

```
network-view-osx/
├── backend/
│   ├── main.go              # mDNS discovery server
│   ├── go.mod               # Go dependencies
│   ├── go.sum               # Go dependency checksums
│   ├── proto/               # Protocol buffer definitions
│   │   └── mdns/v1/
│   │       └── service.proto
│   ├── buf.yaml             # Buf configuration
│   └── buf.gen.yaml         # Buf code generation
├── frontend/
│   ├── src/
│   │   ├── App.svelte       # Main app component
│   │   ├── App.css          # App styling
│   │   └── main.js          # Entry point
│   ├── index.html
│   ├── vite.config.js
│   ├── package.json
│   └── package-lock.json
├── README.md
├── DEVELOPMENT.md           # This file
└── .gitignore
```

## Key Files Explained

### Backend

**main.go**:
- `MDNSServer`: Manages connected clients and broadcasts discovered services
- `startMDNSDiscovery()`: Periodically queries mDNS for services
- `discoverService()`: Queries a specific service type via DNS
- `queryServiceDetails()`: Fetches SRV records for a service
- `queryHostIP()`: Resolves hostname to IP address
- `/health`: Health check endpoint
- `/discover`: Server-Sent Events endpoint for service streaming

### Frontend

**App.svelte**:
- `connectToMDNS()`: Establishes EventSource connection
- `reconnect()`: Reconnects to backend
- Real-time service list with deduplication
- Search and filtering
- Connection status indicator

## API Endpoints

### GET /health
Health check endpoint. Returns `{"status":"ok"}`.

### GET /discover
Server-Sent Events stream for mDNS discovery.

**Response Format**:
```json
{
  "service": {
    "name": "MacBook-Pro",
    "type": "_ssh._tcp.local.",
    "host": "macbook-pro.local",
    "ip": "192.168.1.100",
    "port": 22,
    "timestamp": 1699564800
  },
  "removed": false
}
```

## Troubleshooting

### Backend won't start
- Make sure port 9999 is available
- Check network permissions for mDNS (224.0.0.251:5353)
- Some networks block mDNS multicast

### No services discovered
- Services must announce themselves via mDNS
- Ensure you have services running on the network
- Try with common services like SSH, HTTP, SMB

### Frontend can't connect to backend
- Make sure backend is running on :9999
- Check CORS headers are being sent
- Try `curl http://localhost:9999/health`

## Adding New Service Types

Edit `startMDNSDiscovery()` in `backend/main.go`:

```go
serviceTypes := []string{
    "_http._tcp.local.",
    "_custom._tcp.local.",  // Add here
}
```

Common service types:
- `_workstation._tcp.local.` - Generic workstations
- `_printer._tcp.local.` - Printers
- `_airplay._tcp.local.` - AirPlay devices
- `_companion-link._tcp.local.` - Apple Companion devices

## Performance Notes

- Services are deduplicated by IP:Port:Type combination
- Queries happen every 5 seconds
- EventSource allows 100 pending responses per client
- Seen services are cached to avoid duplicate broadcasts

## Future Enhancements

- [ ] Service-specific metadata (device type, version, features)
- [ ] Service details view with port information
- [ ] Export discovered services as CSV/JSON
- [ ] Service availability history/uptime tracking
- [ ] Custom service type filtering
- [ ] Dark mode
- [ ] Persistent service list with timestamps
