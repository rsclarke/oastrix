// Package defaultresponse implements the core plugin that provides baseline responses for HTTP and DNS.
package defaultresponse

import (
	"context"
	"net"

	"github.com/miekg/dns"
	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/events"
	"github.com/rsclarke/oastrix/internal/plugins"
)

// Plugin provides default responses for HTTP and DNS when no other plugin has handled them.
type Plugin struct {
	publicIP net.IP
	logger   *zap.Logger
}

// New creates a new defaultresponse Plugin with the given public IP address.
func New(publicIP string) *Plugin {
	ip := net.ParseIP(publicIP)
	if ip == nil {
		ip = net.IPv4(127, 0, 0, 1)
	}
	return &Plugin{publicIP: ip}
}

// ID returns the plugin identifier.
func (p *Plugin) ID() string { return "defaultresponse" }

// Init initializes the plugin with the given context.
func (p *Plugin) Init(ctx plugins.InitContext) error {
	p.logger = ctx.Logger.Named("defaultresponse")
	return nil
}

// Priority returns a high value so this plugin runs last.
func (p *Plugin) Priority() int { return 999 }

// OnHTTPResponse sets a default 200 OK response if not already handled.
func (p *Plugin) OnHTTPResponse(_ context.Context, e *events.HTTPEvent) error {
	if e.Resp == nil || e.Resp.Handled {
		return nil
	}
	e.Resp.Status = 200
	e.Resp.Body = []byte("ok")
	e.Resp.Handled = true
	return nil
}

// OnDNSResponse adds an A record pointing to publicIP if not already handled and query is type A.
func (p *Plugin) OnDNSResponse(_ context.Context, e *events.DNSEvent) error {
	if e.Resp == nil || e.Resp.Handled {
		return nil
	}
	if e.Draft == nil || e.Draft.DNS == nil {
		return nil
	}
	if e.Draft.DNS.QType != int(dns.TypeA) {
		return nil
	}

	qname := e.Draft.DNS.QName
	if qname != "" && qname[len(qname)-1] != '.' {
		qname += "."
	}

	rr := &dns.A{
		Hdr: dns.RR_Header{
			Name:   qname,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: p.publicIP,
	}

	e.Resp.Answers = append(e.Resp.Answers, rr)
	e.Resp.Handled = true
	return nil
}
