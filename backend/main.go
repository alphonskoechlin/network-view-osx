package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
)

type MDNSService struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Host      string `json:"host"`
	IP        string `json:"ip"`
	Port      uint16 `json:"port"`
	Timestamp int64  `json:"timestamp"`
}

type DiscoveryResponse struct {
	Service MDNSService `json:"service"`
	Removed bool        `json:"removed"`
}

type MDNSServer struct {
	clients      map[chan *DiscoveryResponse]bool
	mu           sync.RWMutex
	seen         map[string]bool
	currentIface string
}

func NewMDNSServer() *MDNSServer {
	return &MDNSServer{
		clients:      make(map[chan *DiscoveryResponse]bool),
		seen:         make(map[string]bool),
		currentIface: "en5",
	}
}

func (s *MDNSServer) broadcast(response *DiscoveryResponse) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for ch := range s.clients {
		select {
		case ch <- response:
		default:
			// Skip if channel is full
		}
	}
}

func (s *MDNSServer) registerClient(ch chan *DiscoveryResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[ch] = true
}

func (s *MDNSServer) unregisterClient(ch chan *DiscoveryResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, ch)
}

func (s *MDNSServer) Discover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	responseChan := make(chan *DiscoveryResponse, 100)
	s.registerClient(responseChan)
	defer s.unregisterClient(responseChan)
	defer close(responseChan)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Explicitly write status line and headers to the client
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	
	for {
		select {
		case <-r.Context().Done():
			return
		case response := <-responseChan:
			if response != nil {
				data, _ := json.Marshal(response)
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
			}
		}
	}
}

func startMDNSDiscovery(server *MDNSServer, iface string) {
	server.mu.Lock()
	server.currentIface = iface
	server.mu.Unlock()

	// Start proper mDNS browser using hashicorp/mdns library
	go browseMDNSServices(server, iface)

	// Also start mDNS listener to capture multicast responses
	go listenMDNSMulticast(server)

	// And periodic queries to trigger responses
	go func() {
		serviceTypes := []string{
			"_http._tcp.local.",
			"_https._tcp.local.",
			"_ssh._tcp.local.",
			"_sftp._tcp.local.",
			"_smb._tcp.local.",
			"_afpovertcp._tcp.local.",
			"_nfs._tcp.local.",
			"_ldap._tcp.local.",
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			for _, serviceType := range serviceTypes {
				discoverService(server, serviceType)
			}
		}
	}()
}

func browseMDNSServices(server *MDNSServer, iface string) {
	// Service types to browse
	serviceTypes := []string{
		"_http._tcp",
		"_https._tcp",
		"_ssh._tcp",
		"_sftp._tcp",
		"_smb._tcp",
		"_afpovertcp._tcp",
		"_nfs._tcp",
		"_ldap._tcp",
		"_sip._tcp",
		"_xmpp._tcp",
		"_workstation._tcp",
		"_device-info._tcp",
	}

	// Browse each service type
	for _, serviceType := range serviceTypes {
		go browseServiceType(server, serviceType)
	}
}

func browseServiceType(server *MDNSServer, serviceType string) {
	// Set up periodic browsing with a timeout
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Create an mDNS query with a timeout
		entriesChan := make(chan *mdns.ServiceEntry, 4)
		
		go func() {
			for entry := range entriesChan {
				if entry == nil {
					continue
				}

				// Extract service info
				serviceName := entry.Name
				if serviceName == "" {
					serviceName = entry.Host
				}

				// Get IP address - use AddrV4 or AddrV6
				var ip string
				if entry.AddrV4 != nil {
					ip = entry.AddrV4.String()
				} else if entry.AddrV6 != nil {
					ip = entry.AddrV6.String()
				}

				if ip == "" {
					continue
				}

				// Create unique key
				key := fmt.Sprintf("%s:%s:%d", ip, serviceType, entry.Port)

				server.mu.Lock()
				seen := server.seen[key]
				server.mu.Unlock()

				if !seen {
					server.mu.Lock()
					server.seen[key] = true
					server.mu.Unlock()

					// Broadcast the discovered service
					service := &MDNSService{
						Name:      serviceName,
						Type:      "_" + serviceType + ".local.",
						Host:      entry.Host,
						IP:        ip,
						Port:      uint16(entry.Port),
						Timestamp: time.Now().Unix(),
					}

					server.broadcast(&DiscoveryResponse{
						Service: *service,
						Removed: false,
					})

					log.Printf("Discovered service: %s (%s) at %s:%d", serviceName, serviceType, ip, entry.Port)
				}
			}
		}()

		// Browser lookup with 3 second timeout
		mdns.Lookup(serviceType, entriesChan)
		close(entriesChan)
	}
}

func listenMDNSMulticast(server *MDNSServer) {
	// Listen to mDNS multicast traffic on 224.0.0.251:5353
	addr, err := net.ResolveUDPAddr("udp", "224.0.0.251:5353")
	if err != nil {
		log.Printf("Failed to resolve mDNS address: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		log.Printf("Failed to listen on mDNS multicast: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Listening to mDNS multicast traffic on 224.0.0.251:5353")

	buffer := make([]byte, 4096)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from mDNS: %v", err)
			continue
		}

		// Parse DNS message
		msg := new(dns.Msg)
		err = msg.Unpack(buffer[:n])
		if err != nil {
			// Ignore invalid messages
			continue
		}

		// Process answers in the message
		// Note: mDNS can include answers even for unsolicited responses
		for _, ans := range msg.Answer {
			switch record := ans.(type) {
			case *dns.PTR:
				// PTR record points to service instances
				queryServiceDetails(server, record.Ptr, record.Hdr.Name)
			case *dns.SRV:
				// SRV record has hostname and port
				// Extract service name from record name
				parts := strings.Split(record.Hdr.Name, ".")
				if len(parts) >= 2 {
					serviceType := record.Hdr.Name
					ip := resolveHostIP(strings.TrimSuffix(record.Target, "."))
					if ip != "" {
						name := parts[0]
						key := fmt.Sprintf("%s:%s:%d", ip, serviceType, record.Port)
						
						server.mu.Lock()
						seen := server.seen[key]
						server.mu.Unlock()
						
						if !seen {
							server.mu.Lock()
							server.seen[key] = true
							server.mu.Unlock()
							
							service := &MDNSService{
								Name:      name,
								Type:      serviceType,
								Host:      strings.TrimSuffix(record.Target, "."),
								IP:        ip,
								Port:      record.Port,
								Timestamp: time.Now().Unix(),
							}
							
							server.broadcast(&DiscoveryResponse{
								Service: *service,
								Removed: false,
							})
						}
					}
				}
			}
		}
	}
}

func discoverService(server *MDNSServer, serviceType string) {
	// Query using DNS protocol to mDNS multicast address
	// Note: This uses standard DNS query mechanism which may have limitations
	// on some networks. For a more robust approach, consider using a dedicated
	// mDNS browser library.
	
	m := new(dns.Msg)
	m.SetQuestion(serviceType, dns.TypePTR)
	m.RecursionDesired = false

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = 500 * time.Millisecond // Reduce timeout for multicast
	c.SingleInflight = false

	// Send to mDNS multicast address
	// Note: mDNS may not respond to unicast queries, only multicast listeners
	in, _, err := c.Exchange(m, "224.0.0.251:5353")
	if err != nil {
		// Expected - multicast queries often timeout
		return
	}

	if in == nil {
		return
	}

	for _, ans := range in.Answer {
		if ptr, ok := ans.(*dns.PTR); ok {
			queryServiceDetails(server, ptr.Ptr, serviceType)
		}
	}
}

func queryServiceDetails(server *MDNSServer, serviceName string, serviceType string) {
	// Query for SRV record
	srvMsg := new(dns.Msg)
	srvMsg.SetQuestion(serviceName, dns.TypeSRV)
	srvMsg.RecursionDesired = false

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = 1 * time.Second

	srvIn, _, srvErr := c.Exchange(srvMsg, "224.0.0.251:5353")
	if srvErr != nil {
		return
	}

	if srvIn == nil {
		return
	}

	for _, srvAns := range srvIn.Answer {
		if srv, ok := srvAns.(*dns.SRV); ok {
			queryHostIP(server, srv.Target, serviceName, serviceType, srv.Port)
		}
	}
}

func queryHostIP(server *MDNSServer, host string, serviceName string, serviceType string, port uint16) {
	// Clean up host name
	hostname := strings.TrimSuffix(host, ".")

	// Try to resolve via mDNS
	ip := resolveHostIP(hostname)
	if ip == "" {
		return
	}

	// Extract service name
	name := strings.Split(serviceName, ".")[0]

	// Create unique key to avoid duplicates
	key := fmt.Sprintf("%s:%s:%d", ip, serviceType, port)

	server.mu.Lock()
	if server.seen[key] {
		server.mu.Unlock()
		return
	}
	server.seen[key] = true
	server.mu.Unlock()

	service := &MDNSService{
		Name:      name,
		Type:      serviceType,
		Host:      hostname,
		IP:        ip,
		Port:      port,
		Timestamp: time.Now().Unix(),
	}

	response := &DiscoveryResponse{
		Service: *service,
		Removed: false,
	}

	server.broadcast(response)
}

func resolveHostIP(hostname string) string {
	// Try A record first
	m := new(dns.Msg)
	m.SetQuestion(hostname+".", dns.TypeA)
	m.RecursionDesired = false

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = 1 * time.Second

	in, _, err := c.Exchange(m, "224.0.0.251:5353")
	if err == nil && in != nil {
		for _, ans := range in.Answer {
			if a, ok := ans.(*dns.A); ok {
				return a.A.String()
			}
		}
	}

	// Fallback to regular DNS resolution
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return ""
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String()
		}
	}

	return ""
}

func getNetworkInterfaces() ([]map[string]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []map[string]string
	for _, iface := range interfaces {
		// Only include up interfaces with addresses
		if iface.Flags&net.FlagUp != 0 {
			result = append(result, map[string]string{
				"name": iface.Name,
				"mtu":  fmt.Sprintf("%d", iface.MTU),
			})
		}
	}
	return result, nil
}

func main() {
	port := flag.String("port", "9999", "Port to listen on")
	bindAddr := flag.String("bind", "", "IP address to bind to (default: all interfaces)")
	iface := flag.String("iface", "en5", "Network interface for mDNS discovery (default: en5)")
	flag.Parse()

	server := NewMDNSServer()
	startMDNSDiscovery(server, *iface)

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// API endpoint for getting available network interfaces
	mux.HandleFunc("/api/interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		interfaces, err := getNetworkInterfaces()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"interfaces": interfaces,
			"current":    server.currentIface,
		}
		data, _ := json.Marshal(response)
		fmt.Fprint(w, string(data))
	})

	// API endpoint for setting network interface
	mux.HandleFunc("/api/interfaces/set", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}

		ifaceName, ok := req["interface"]
		if !ok || ifaceName == "" {
			http.Error(w, `{"error":"interface name required"}`, http.StatusBadRequest)
			return
		}

		// Verify interface exists
		ifaces, _ := getNetworkInterfaces()
		found := false
		for _, iface := range ifaces {
			if iface["name"] == ifaceName {
				found = true
				break
			}
		}

		if !found {
			http.Error(w, fmt.Sprintf(`{"error":"interface %s not found"}`, ifaceName), http.StatusNotFound)
			return
		}

		// Update current interface and restart discovery
		server.mu.Lock()
		server.currentIface = ifaceName
		server.seen = make(map[string]bool) // Reset seen services
		server.mu.Unlock()

		fmt.Fprintf(w, `{"status":"ok","interface":"%s"}`, ifaceName)
	})

	// API endpoint for discovery
	mux.HandleFunc("/discover", server.Discover)

	// Serve frontend files with SPA support
	distPath := filepath.Join("..", "frontend", "dist")
	if info, err := os.Stat(distPath); err == nil && info.IsDir() {
		// Create custom handler for SPA - serve index.html for root and missing files
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// API routes should be handled by their specific handlers
			if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/discover") || strings.HasPrefix(r.URL.Path, "/health") {
				http.NotFound(w, r)
				return
			}
			
			// Try to serve the requested file
			fullPath := filepath.Join(distPath, filepath.Clean(r.URL.Path))
			
			// Security: prevent directory traversal
			if !strings.HasPrefix(fullPath, distPath) {
				http.NotFound(w, r)
				return
			}
			
			// Check if file exists
			if _, err := os.Stat(fullPath); err == nil {
				// File exists, serve it
				http.ServeFile(w, r, fullPath)
			} else if r.URL.Path == "/" {
				// Root path, serve index.html
				http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
			} else {
				// File doesn't exist, serve index.html (for SPA routing)
				w.Header().Set("Content-Type", "text/html")
				http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
			}
		})
		log.Printf("Serving frontend from %s", distPath)
	}

	corsHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	var listenAddr string
	if *bindAddr != "" {
		listenAddr = *bindAddr + ":" + *port
	} else {
		listenAddr = ":" + *port
	}

	log.Printf("Starting mDNS discovery server on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, corsHandler(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
