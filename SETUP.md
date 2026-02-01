# Setup Instructions

## Prerequisites

### macOS
- Go 1.21+ ([download](https://go.dev/dl/))
- Node.js 18+ ([download](https://nodejs.org/))
- npm (comes with Node.js)

### Ubuntu/Linux
```bash
sudo apt-get update
sudo apt-get install -y golang-go nodejs npm
```

## Quick Start

### 1. Clone Repository
```bash
git clone https://github.com/alphonskoechlin/network-view-osx.git
cd network-view-osx
```

### 2. Backend Setup
```bash
cd backend
go mod download
```

### 3. Frontend Setup
```bash
cd ../frontend
npm install
```

## Running the Application

### Start Backend (Terminal 1)
```bash
cd backend
go run main.go
```

Expected output:
```
Starting mDNS discovery server on :8080
```

### Start Frontend (Terminal 2)
```bash
cd frontend
npm run dev
```

Expected output:
```
  VITE v5.0.2  ready in XXX ms

  ➜  Local:   http://localhost:5173/
  ➜  press h to show help
```

### Open in Browser
Navigate to: http://localhost:5173

## Building for Production

### Backend
```bash
cd backend
go build -o network-view-osx
./network-view-osx
```

### Frontend
```bash
cd frontend
npm run build
# Output in frontend/dist/
```

## Docker Setup (Optional)

### Build Docker Image
```bash
docker build -t network-view-osx .
docker run -p 8080:8080 -p 5173:5173 network-view-osx
```

## System Requirements

- **macOS**: 10.12+
- **Linux**: Ubuntu 18.04+ or equivalent
- **Windows**: WSL2 with Ubuntu
- **Network**: Access to mDNS multicast (224.0.0.251:5353)

## Firewall/Network Configuration

### macOS
mDNS should work out of the box. If not:
1. Check System Preferences > Security & Privacy > Firewall
2. Add Go binary to firewall allowlist

### Linux
If services aren't discovered:
```bash
# Check if mDNS is available
ping -I eth0 224.0.0.251

# Install avahi if needed
sudo apt-get install -y avahi-daemon
sudo systemctl start avahi-daemon
```

## Troubleshooting

### "Connection to mDNS service lost"
- Verify backend is running: `curl http://localhost:8080/health`
- Check port 8080 is not in use: `lsof -i :8080`

### No services discovered
- Verify mDNS is working: `dns-sd -B _services._dns-sd._udp local.`
- Ensure you have mDNS services on your network
- Try services like SSH (`_ssh._tcp.local.`)

### Frontend shows "Disconnected"
- Check backend is running on :8080
- Try: `curl http://localhost:8080/health`
- Check browser console for errors (F12)

### Port Already in Use
```bash
# Free port 8080
lsof -i :8080
kill -9 <PID>

# Or use different port in backend/main.go
```

## Next Steps

- Read [README.md](README.md) for feature overview
- Read [DEVELOPMENT.md](DEVELOPMENT.md) for architecture details
- Check [GitHub Issues](https://github.com/alphonskoechlin/network-view-osx/issues) for known issues
