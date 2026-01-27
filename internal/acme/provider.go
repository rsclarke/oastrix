package acme

import (
	"context"
	"strings"

	"github.com/libdns/libdns"
)

var _ libdns.RecordAppender = (*Provider)(nil)
var _ libdns.RecordDeleter = (*Provider)(nil)

type Provider struct {
	Store *TXTStore
}

func (p *Provider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	for _, r := range recs {
		rr := r.RR()
		if strings.EqualFold(rr.Type, "TXT") {
			fqdn := absoluteName(zone, rr.Name)
			p.Store.Add(fqdn, rr.Data)
		}
	}
	return recs, nil
}

func (p *Provider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	for _, r := range recs {
		rr := r.RR()
		if strings.EqualFold(rr.Type, "TXT") {
			fqdn := absoluteName(zone, rr.Name)
			p.Store.Remove(fqdn, rr.Data)
		}
	}
	return recs, nil
}

func absoluteName(zone, name string) string {
	zone = strings.TrimSuffix(strings.ToLower(zone), ".")
	name = strings.TrimSuffix(strings.ToLower(name), ".")

	if strings.HasSuffix(name, zone) {
		return name
	}

	if name == "" || name == "." {
		return zone
	}

	return name + "." + zone
}
