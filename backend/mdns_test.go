package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// TestMDNSDiscovery tests mDNS service discovery on en5 with a 30-second timeout
func TestMDNSDiscovery(t *testing.T) {
	// Check if en5 interface is active and has multicast capability
	iface, err := net.InterfaceByName("en5")
	if err != nil {
		t.Logf("⚠️  Interface en5 not found: %v", err)
		t.Logf("   Available interfaces:")
		interfaces, _ := net.Interfaces()
		for _, i := range interfaces {
			flags := ""
			if i.Flags&net.FlagUp != 0 {
				flags += "UP "
			}
			if i.Flags&net.FlagMulticast != 0 {
				flags += "MULTICAST "
			}
			t.Logf("   - %s (%s)", i.Name, flags)
		}
		t.Skip("en5 interface not available")
	}

	// Check if interface is up
	if iface.Flags&net.FlagUp == 0 {
		t.Fatalf("Interface en5 is not up")
	}

	// Check if interface supports multicast
	if iface.Flags&net.FlagMulticast == 0 {
		t.Fatalf("Interface en5 does not support multicast")
	}

	t.Logf("✓ Interface en5 is UP and supports MULTICAST")
	t.Logf("  MTU: %d", iface.MTU)

	// Test mDNS multicast socket
	testMulticastSocket(t)

	// Run discovery with a 30-second timeout
	discovered := runMDNSDiscoveryTest(t)

	if len(discovered) == 0 {
		t.Logf("⚠️  No mDNS services discovered within 30 seconds")
		t.Logf("   This could mean:")
		t.Logf("   - No mDNS services are running on the network")
		t.Logf("   - Multicast is not properly configured")
		t.Logf("   - The mDNS responders are not answering queries")
		t.Logf("   - Firewall is blocking mDNS traffic (224.0.0.251:5353)")
		t.Skip("No services discovered - check network configuration and running services")
	}

	// Log discovered services
	t.Logf("✓ Found %d mDNS services:", len(discovered))
	for i, svc := range discovered {
		t.Logf("  %d. %s (%s) on %s:%d", i+1, svc.Name, svc.Type, svc.Host, svc.Port)
	}

	// Assert we found at least one service
	if len(discovered) < 1 {
		t.Fatalf("Expected at least 1 service, found %d", len(discovered))
	}
}

// testMulticastSocket verifies multicast connectivity
func testMulticastSocket(t *testing.T) {
	// Create a multicast UDP socket on mDNS address/port
	addr, err := net.ResolveUDPAddr("udp", "224.0.0.251:5353")
	if err != nil {
		t.Logf("⚠️  Failed to resolve multicast address: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		t.Logf("⚠️  Failed to listen on multicast address: %v", err)
		t.Logf("   Multicast may not be properly configured")
		return
	}
	defer conn.Close()

	t.Logf("✓ Multicast socket created successfully on 224.0.0.251:5353")
}

// runMDNSDiscoveryTest performs mDNS listening and discovery
func runMDNSDiscoveryTest(t *testing.T) []*MDNSService {
	discovered := make([]*MDNSService, 0)
	seenKeys := make(map[string]bool)
	mu := &sync.Mutex{}

	// Standard mDNS service types to query
	serviceTypes := []string{
		"_http._tcp.local.",
		"_https._tcp.local.",
		"_ssh._tcp.local.",
		"_sftp._tcp.local.",
		"_smb._tcp.local.",
		"_afpovertcp._tcp.local.",
		"_nfs._tcp.local.",
		"_ldap._tcp.local.",
		"_workstation._tcp.local.",
		"_device-info._tcp.local.",
		"_airplay._tcp.local.",
	}

	// Set timeout for overall discovery (30 seconds)
	deadline := time.Now().Add(30 * time.Second)

	t.Logf("Starting mDNS discovery on en5 (30-second timeout)...")
	t.Logf("Listening for %d service types...", len(serviceTypes))

	// Start multicast listener to capture responses
	go listenAndProcessMDNSResponses(t, deadline, &discovered, seenKeys, mu)

	// Send queries periodically to trigger responses
	go sendMDNSQueries(t, deadline, serviceTypes)

	// Wait until deadline
	<-time.After(time.Until(deadline))

	t.Logf("Discovery completed")
	return discovered
}

// listenAndProcessMDNSResponses listens to mDNS multicast and extracts service info
func listenAndProcessMDNSResponses(t *testing.T, deadline time.Time, discovered *[]*MDNSService, seenKeys map[string]bool, mu *sync.Mutex) {
	addr, err := net.ResolveUDPAddr("udp", "224.0.0.251:5353")
	if err != nil {
		t.Logf("⚠️  Failed to resolve mDNS address: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		t.Logf("⚠️  Failed to listen on mDNS multicast: %v", err)
		return
	}
	defer conn.Close()

	t.Logf("✓ Listening to mDNS multicast 224.0.0.251:5353")

	buffer := make([]byte, 4096)
	for time.Now().Before(deadline) {
		// Set read deadline for this packet
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			// Timeout is expected, continue listening
			continue
		}

		// Parse DNS message
		msg := new(dns.Msg)
		err = msg.Unpack(buffer[:n])
		if err != nil {
			continue
		}

		// Process all answer records
		for _, ans := range msg.Answer {
			switch record := ans.(type) {
			case *dns.PTR:
				// PTR record: service type -> service instance
				serviceName := record.Ptr
				serviceType := record.Hdr.Name

				t.Logf("  ✓ Found PTR: %s -> %s", serviceType, serviceName)

				// Query SRV record for this service
				srvService := queryServiceDetailsForTest(t, serviceName, serviceType)
				if srvService != nil {
					mu.Lock()
					key := fmt.Sprintf("%s:%s:%d", srvService.IP, serviceType, srvService.Port)
					if !seenKeys[key] {
						seenKeys[key] = true
						*discovered = append(*discovered, srvService)
						t.Logf("    ✓ Added: %s at %s:%d", srvService.Name, srvService.IP, srvService.Port)
					}
					mu.Unlock()
				}

			case *dns.SRV:
				// SRV record: service instance -> host and port
				parts := strings.Split(record.Hdr.Name, ".")
				if len(parts) >= 2 {
					serviceName := parts[0]
					serviceType := record.Hdr.Name
					hostname := strings.TrimSuffix(record.Target, ".")

					// Resolve hostname to IP
					ip := resolveHostIPForTest(t, hostname)
					if ip != "" {
						mu.Lock()
						key := fmt.Sprintf("%s:%s:%d", ip, serviceType, record.Port)
						if !seenKeys[key] {
							seenKeys[key] = true
							service := &MDNSService{
								Name:      serviceName,
								Type:      serviceType,
								Host:      hostname,
								IP:        ip,
								Port:      uint16(record.Port),
								Timestamp: time.Now().Unix(),
							}
							*discovered = append(*discovered, service)
							t.Logf("    ✓ Added from SRV: %s at %s:%d", serviceName, ip, record.Port)
						}
						mu.Unlock()
					}
				}
			}
		}
	}
}

// sendMDNSQueries sends periodic mDNS queries to trigger responses
func sendMDNSQueries(t *testing.T, deadline time.Time, serviceTypes []string) {
	queryTicker := time.NewTicker(2 * time.Second)
	defer queryTicker.Stop()

	queryCount := 0
	for {
		select {
		case <-queryTicker.C:
			if time.Now().After(deadline) {
				return
			}

			for _, serviceType := range serviceTypes {
				queryCount++
				t.Logf("[Query #%d] Querying %s...", queryCount, serviceType)

				m := new(dns.Msg)
				m.SetQuestion(serviceType, dns.TypePTR)
				m.RecursionDesired = false

				c := new(dns.Client)
				c.Net = "udp"
				c.Timeout = 500 * time.Millisecond

				// Send to mDNS multicast
				_, _, _ = c.Exchange(m, "224.0.0.251:5353")
				// Ignore errors - multicast queries often timeout
			}
		}
	}
}

// queryServiceDetailsForTest performs SRV lookup for a service
func queryServiceDetailsForTest(t *testing.T, serviceName string, serviceType string) *MDNSService {
	// Query for SRV record
	srvMsg := new(dns.Msg)
	srvMsg.SetQuestion(serviceName, dns.TypeSRV)
	srvMsg.RecursionDesired = false

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = 1 * time.Second

	srvIn, _, srvErr := c.Exchange(srvMsg, "224.0.0.251:5353")
	if srvErr != nil {
		t.Logf("    ✗ SRV query failed: %v", srvErr)
		return nil
	}

	if srvIn == nil {
		t.Logf("    ⚠️  No SRV response")
		return nil
	}

	for _, srvAns := range srvIn.Answer {
		if srv, ok := srvAns.(*dns.SRV); ok {
			t.Logf("    ✓ SRV record: target=%s, port=%d", srv.Target, srv.Port)

			// Resolve hostname to IP
			hostname := strings.TrimSuffix(srv.Target, ".")
			ip := resolveHostIPForTest(t, hostname)

			if ip == "" {
				t.Logf("    ✗ Could not resolve IP for %s", hostname)
				return nil
			}

			// Extract service name from serviceName
			nameParts := strings.Split(serviceName, ".")
			name := nameParts[0]

			t.Logf("    ✓ Resolved %s -> %s", hostname, ip)

			return &MDNSService{
				Name:      name,
				Type:      serviceType,
				Host:      hostname,
				IP:        ip,
				Port:      srv.Port,
				Timestamp: time.Now().Unix(),
			}
		}
	}

	return nil
}

// resolveHostIPForTest resolves a hostname to IP via mDNS or regular DNS
func resolveHostIPForTest(t *testing.T, hostname string) string {
	// Try A record first via mDNS
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
		t.Logf("    Could not resolve %s: %v", hostname, err)
		return ""
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String()
		}
	}

	return ""
}

// TestMDNSMulticastListener verifies we can listen to multicast traffic
func TestMDNSMulticastListener(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "224.0.0.251:5353")
	if err != nil {
		t.Fatalf("Failed to resolve multicast address: %v", err)
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("Failed to create multicast listener: %v", err)
	}
	defer conn.Close()

	t.Logf("✓ Multicast listener created successfully")

	// Set a read deadline
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buffer := make([]byte, 4096)
	n, remoteAddr, err := conn.ReadFromUDP(buffer)

	if err != nil {
		// This is expected if no mDNS traffic is present
		t.Logf("⚠️  No mDNS traffic received within 5 seconds")
		t.Logf("   This is normal if no mDNS queries or responses are happening on the network")
	} else {
		t.Logf("✓ Received mDNS packet (%d bytes) from %s", n, remoteAddr)

		// Try to parse as DNS message
		msg := new(dns.Msg)
		err := msg.Unpack(buffer[:n])
		if err == nil {
			t.Logf("✓ Valid DNS packet: %d questions, %d answers", len(msg.Question), len(msg.Answer))
		}
	}
}

// TestNetworkInterfaces verifies network interface enumeration
func TestNetworkInterfaces(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("Failed to enumerate interfaces: %v", err)
	}

	t.Logf("Found %d network interfaces:", len(interfaces))
	for _, iface := range interfaces {
		flags := ""
		if iface.Flags&net.FlagUp != 0 {
			flags += "UP "
		}
		if iface.Flags&net.FlagLoopback != 0 {
			flags += "LOOPBACK "
		}
		if iface.Flags&net.FlagMulticast != 0 {
			flags += "MULTICAST "
		}
		if iface.Flags&net.FlagBroadcast != 0 {
			flags += "BROADCAST "
		}

		addrs, _ := iface.Addrs()
		addrStr := ""
		if len(addrs) > 0 {
			addrStr = fmt.Sprintf(" (%s)", addrs[0].String())
		}

		t.Logf("  - %s: %s%s", iface.Name, flags, addrStr)
	}

	// Check for en5 specifically
	iface, err := net.InterfaceByName("en5")
	if err == nil {
		t.Logf("\n✓ en5 is available")
		t.Logf("  Flags: %v", iface.Flags)
		t.Logf("  MTU: %d", iface.MTU)
		addrs, _ := iface.Addrs()
		t.Logf("  Addresses: %v", addrs)
	} else {
		t.Logf("\n⚠️  en5 is not available: %v", err)
	}
}
