package server

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/rsclarke/oastrix/internal/acme"
	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/logging"
	"go.uber.org/zap"
)

// DNSServer handles DNS queries and records interactions.
type DNSServer struct {
	DB        *sql.DB
	Domain    string
	PublicIP  string // IP address to return for ns1.<domain> and A queries
	TXTStore  *acme.TXTStore
	Logger    *zap.Logger
	udpServer *dns.Server
	tcpServer *dns.Server
}

// Start begins listening for DNS queries on the specified UDP and TCP ports.
func (s *DNSServer) Start(udpPort, tcpPort int) error {
	handler := dns.HandlerFunc(s.handleDNS)

	s.udpServer = &dns.Server{
		Addr:    fmt.Sprintf(":%d", udpPort),
		Net:     "udp",
		Handler: handler,
	}

	s.tcpServer = &dns.Server{
		Addr:    fmt.Sprintf(":%d", tcpPort),
		Net:     "tcp",
		Handler: handler,
	}

	udpErrCh := make(chan error, 1)
	tcpErrCh := make(chan error, 1)

	go func() {
		s.Logger.Info("starting dns server", logging.Net("udp"), logging.Port(udpPort))
		if err := s.udpServer.ListenAndServe(); err != nil {
			udpErrCh <- err
		}
		close(udpErrCh)
	}()

	go func() {
		s.Logger.Info("starting dns server", logging.Net("tcp"), logging.Port(tcpPort))
		if err := s.tcpServer.ListenAndServe(); err != nil {
			tcpErrCh <- err
		}
		close(tcpErrCh)
	}()

	timeout := time.After(100 * time.Millisecond)
	for i := 0; i < 2; i++ {
		select {
		case err := <-udpErrCh:
			if err != nil {
				return fmt.Errorf("UDP DNS server failed to start: %w", err)
			}
		case err := <-tcpErrCh:
			if err != nil {
				return fmt.Errorf("TCP DNS server failed to start: %w", err)
			}
		case <-timeout:
			return nil
		}
	}

	return nil
}

// Shutdown gracefully stops the DNS servers.
func (s *DNSServer) Shutdown(ctx context.Context) {
	if s.udpServer != nil {
		if err := s.udpServer.ShutdownContext(ctx); err != nil {
			s.Logger.Warn("dns udp shutdown error", zap.Error(err))
		}
	}
	if s.tcpServer != nil {
		if err := s.tcpServer.ShutdownContext(ctx); err != nil {
			s.Logger.Warn("dns tcp shutdown error", zap.Error(err))
		}
	}
}

func (s *DNSServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	protocol := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		protocol = "tcp"
	}

	remoteIP, remotePort := parseRemoteAddr(w.RemoteAddr())

	for _, q := range r.Question {
		qname := strings.ToLower(strings.TrimSuffix(q.Name, "."))

		// Handle SOA queries for the domain (required for ACME zone discovery)
		if q.Qtype == dns.TypeSOA {
			if qname == s.Domain || strings.HasSuffix(qname, "."+s.Domain) {
				soa := &dns.SOA{
					Hdr:     dns.RR_Header{Name: s.Domain + ".", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 300},
					Ns:      "ns1." + s.Domain + ".",
					Mbox:    "hostmaster." + s.Domain + ".",
					Serial:  1,
					Refresh: 3600,
					Retry:   600,
					Expire:  604800,
					Minttl:  1, // Low TTL to minimize ACME challenge caching issues
				}
				m.Answer = append(m.Answer, soa)
				continue
			}
		}

		// Handle NS queries for the domain
		if q.Qtype == dns.TypeNS && qname == s.Domain {
			ns := &dns.NS{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
				Ns:  "ns1." + s.Domain + ".",
			}
			m.Answer = append(m.Answer, ns)
			continue
		}

		// Handle queries for ns1.<domain> (required for ACME to resolve nameserver)
		if qname == "ns1."+s.Domain {
			if q.Qtype == dns.TypeA && s.PublicIP != "" {
				rr := &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
					A:   net.ParseIP(s.PublicIP),
				}
				m.Answer = append(m.Answer, rr)
			}
			// For other types (AAAA, etc.), return empty answer (no error)
			continue
		}

		// Handle A queries for the base domain (required for API server access)
		if qname == s.Domain && q.Qtype == dns.TypeA && s.PublicIP != "" {
			rr := &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   net.ParseIP(s.PublicIP),
			}
			m.Answer = append(m.Answer, rr)
			continue
		}

		if q.Qtype == dns.TypeTXT && s.TXTStore != nil {
			normalizedName := acme.NormalizeName(q.Name)
			values := s.TXTStore.Get(normalizedName)
			if len(values) > 0 {
				for _, value := range values {
					rr := &dns.TXT{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 1},
						Txt: []string{value},
					}
					m.Answer = append(m.Answer, rr)
				}
				continue
			}
		}

		token := extractTokenFromQName(qname, s.Domain)

		if token == "" {
			m.Rcode = dns.RcodeNameError
			continue
		}

		tok, err := db.GetTokenByValue(s.DB, token)
		if err != nil {
			s.Logger.Error("lookup token failed", logging.Token(token), zap.Error(err))
			m.Rcode = dns.RcodeNameError
			continue
		}
		if tok == nil {
			s.Logger.Debug("unknown token", logging.Token(token), logging.QName(qname))
			m.Rcode = dns.RcodeNameError
			continue
		}

		summary := fmt.Sprintf("%s %s %s", dns.TypeToString[q.Qtype], qname, protocol)

		rd := 0
		if r.RecursionDesired {
			rd = 1
		}

		interactionID, err := db.CreateInteraction(s.DB, tok.ID, "dns", remoteIP, remotePort, false, summary)
		if err != nil {
			s.Logger.Error("create dns interaction failed", zap.Error(err))
			continue
		}

		err = db.CreateDNSInteraction(s.DB, interactionID, qname, int(q.Qtype), int(q.Qclass), rd, r.Opcode, int(r.Id), protocol)
		if err != nil {
			s.Logger.Error("create dns interaction details failed", zap.Error(err))
		}

		if q.Qtype == dns.TypeA {
			rr := &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(s.PublicIP),
			}
			m.Answer = append(m.Answer, rr)
		}
	}

	w.WriteMsg(m)
}

func extractTokenFromQName(qname, domain string) string {
	domain = strings.ToLower(domain)

	if !strings.HasSuffix(qname, "."+domain) && qname != domain {
		return ""
	}

	if qname == domain {
		return ""
	}

	subdomain := strings.TrimSuffix(qname, "."+domain)
	parts := strings.Split(subdomain, ".")
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

func parseRemoteAddr(addr net.Addr) (string, int) {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP.String(), a.Port
	case *net.TCPAddr:
		return a.IP.String(), a.Port
	default:
		return addr.String(), 0
	}
}
