package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/rsclarke/oastrix/internal/events"
	"github.com/rsclarke/oastrix/internal/plugins"
	"go.uber.org/zap"
)

// HTTPServer handles HTTP requests and records interactions.
type HTTPServer struct {
	Pipeline *plugins.Pipeline
	Domain   string
	PublicIP string
	Logger   *zap.Logger
}

// ExtractToken extracts an OAST token from the request host or path.
func ExtractToken(r *http.Request, domain string) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")

	if strings.HasSuffix(host, "."+domain) {
		subdomain := strings.TrimSuffix(host, "."+domain)
		if dotIdx := strings.LastIndex(subdomain, "."); dotIdx != -1 {
			subdomain = subdomain[dotIdx+1:]
		}
		if subdomain != "" {
			return subdomain
		}
	}

	path := r.URL.Path
	if strings.HasPrefix(path, "/oast/") {
		remaining := strings.TrimPrefix(path, "/oast/")
		if slashIdx := strings.Index(remaining, "/"); slashIdx != -1 {
			remaining = remaining[:slashIdx]
		}
		if remaining != "" {
			return remaining
		}
	}

	return ""
}

func (s *HTTPServer) isValidHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")

	if strings.HasSuffix(host, "."+s.Domain) {
		return true
	}

	if host == s.Domain {
		return true
	}

	if s.PublicIP != "" && host == s.PublicIP {
		return true
	}

	return false
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle ACME HTTP-01 challenges for IP certificate acquisition
	if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
		if certmagic.DefaultACME.HandleHTTPChallenge(w, r) {
			return
		}
	}

	if !s.isValidHost(r.Host) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	token := ExtractToken(r, s.Domain)
	if token == "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	remoteIP, remotePortStr, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
		remotePortStr = "0"
	}
	remotePort, _ := strconv.Atoi(remotePortStr)

	tls := r.TLS != nil

	scheme := "http"
	if tls {
		scheme = "https"
	}

	summary := fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, r.Proto)

	headers := make(map[string][]string)
	for k, v := range r.Header {
		headers[k] = v
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.Logger.Warn("read body failed", zap.Error(err))
		body = nil
	}

	draft := &events.InteractionDraft{
		TokenValue: token,
		Kind:       events.KindHTTP,
		OccurredAt: time.Now().Unix(),
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		TLS:        tls,
		Summary:    summary,
		HTTP: &events.HTTPDraft{
			Method:  r.Method,
			Scheme:  scheme,
			Host:    r.Host,
			Path:    r.URL.Path,
			Query:   r.URL.RawQuery,
			Proto:   r.Proto,
			Headers: headers,
			Body:    body,
		},
		Attributes: make(map[string]any),
	}

	resp := &events.HTTPResponsePlan{
		Status:  200,
		Headers: make(map[string]string),
		Body:    []byte("ok"),
	}

	e := &events.HTTPEvent{
		Event:   events.Event{Draft: draft},
		Req:     r,
		Resp:    resp,
		Scratch: make(map[string]any),
	}

	if err := s.Pipeline.ProcessHTTP(r.Context(), e); err != nil {
		s.Logger.Error("pipeline error", zap.Error(err))
	}

	for k, v := range e.Resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(e.Resp.Status)
	_, _ = w.Write(e.Resp.Body)
}
