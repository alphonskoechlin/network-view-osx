# Network View macOS

A real-time mDNS service discovery application for macOS with a Go backend and Svelte frontend.

## Features

- **Real-time mDNS Discovery**: Streams discovered services to the frontend in real-time
- **Connect RPC**: Uses connectrpc for efficient gRPC communication
- **Carbon Design System**: Professional, responsive UI
- **Service Details**: Shows name, type, host, IP, and port for each discovered service
- **Search & Filter**: Built-in search functionality
- **Connection Status**: Visual indicator of backend connection

## Project Structure

```
network-view-osx/
├── backend/
│   ├── main.go
│   ├── go.mod
│   └── proto/
│       └── mdns/v1/
│           └── service.proto
├── frontend/
│   ├── src/
│   │   ├── App.svelte
│   │   ├── App.css
│   │   └── main.js
│   ├── index.html
│   ├── vite.config.js
│   ├── package.json
│   └── package-lock.json
└── README.md
```

## Prerequisites

- Go 1.21+
- Node.js 18+
- npm

## Setup & Installation

### Backend

1. Navigate to the backend directory:
   ```bash
   cd backend
   ```

2. Generate protobuf code:
   ```bash
   go install github.com/bufbuild/buf/cmd/buf@latest
   go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   buf generate
   ```

3. Download dependencies:
   ```bash
   go mod download
   ```

4. Run the server:
   ```bash
   go run main.go
   ```
   The server will start on `http://localhost:8080`

### Frontend

1. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```

2. Install dependencies:
   ```bash
   npm install
   ```

3. Start the development server:
   ```bash
   npm run dev
   ```
   The frontend will be available at `http://localhost:5173`

## Architecture

### Backend (Go)

- **mDNS Discovery**: Uses the `miekg/dns` library to query mDNS services
- **Connect RPC**: Implements `connectrpc.com/connect` for streaming gRPC services
- **Service Broadcasting**: Maintains client connections and broadcasts discovered services

### Frontend (Svelte + Vite)

- **Carbon Components**: Uses `carbon-components-svelte` for UI
- **Real-time Streaming**: Connects to backend via Connect RPC EventSource
- **Responsive Design**: Mobile-friendly interface with Carbon Design System

## How It Works

1. **Backend Discovery**:
   - Queries mDNS multicast address (224.0.0.251:5353)
   - Looks for common service types (_http, _ssh, _smb, _afpovertcp)
   - Periodically re-queries to catch new services

2. **Streaming**:
   - Each connected client receives a stream of discovered services
   - Services are broadcast to all connected clients in real-time

3. **Frontend Display**:
   - Connects to the backend stream on page load
   - Displays unique services in a data table
   - Supports searching and filtering
   - Shows connection status indicator

## Development

### Generate Proto Code

When updating `proto/mdns/v1/service.proto`:

```bash
cd backend
buf generate
```

### Build for Production

**Frontend:**
```bash
cd frontend
npm run build
```

**Backend:**
```bash
cd backend
go build -o network-view-osx
```

## Testing

Try discovering services on your network:

```bash
# Query for HTTP services
dns q _http._tcp.local PTR @224.0.0.251
```

## License

MIT

## Author

Alphons Koechlin
