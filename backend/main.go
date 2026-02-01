package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

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
	clients map[chan *DiscoveryResponse]bool
	mu      sync.RWMutex
	seen    map[string]bool
}

func NewMDNSServer() *MDNSServer {
	return &MDNSServer{
		clients: make(map[chan *DiscoveryResponse]bool),
		seen:    make(map[string]bool),
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

func startMDNSDiscovery(server *MDNSServer) {
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
			"_sip._tcp.local.",
			"_xmpp._tcp.local.",
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

func discoverService(server *MDNSServer, serviceType string) {
	m := new(dns.Msg)
	m.SetQuestion(serviceType, dns.TypePTR)
	m.RecursionDesired = false

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = 1 * time.Second

	in, _, err := c.Exchange(m, "224.0.0.251:5353")
	if err != nil {
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

func main() {
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	server := NewMDNSServer()
	startMDNSDiscovery(server)

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// CORS middleware
	mux.HandleFunc("/discover", server.Discover)

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

	addr := ":" + *port
	log.Printf("Starting mDNS discovery server on %s", addr)
	if err := http.ListenAndServe(addr, corsHandler(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
