package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/logging"
	"go.uber.org/zap"
)

type HTTPServer struct {
	DB     *sql.DB
	Domain string
	Logger *zap.Logger
}

func ExtractToken(r *http.Request, domain string) string {
	host := r.Host
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

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

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := ExtractToken(r, s.Domain)
	if token == "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	tok, err := db.GetTokenByValue(s.DB, token)
	if err != nil {
		s.Logger.Error("lookup token failed", logging.Token(token), zap.Error(err))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}
	if tok == nil {
		s.Logger.Debug("unknown token", logging.Token(token))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	remoteIP, remotePortStr, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
		remotePortStr = "0"
	}
	var remotePort int
	fmt.Sscanf(remotePortStr, "%d", &remotePort)

	tls := r.TLS != nil

	scheme := "http"
	if tls {
		scheme = "https"
	}

	summary := fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, r.Proto)

	interactionID, err := db.CreateInteraction(s.DB, tok.ID, "http", remoteIP, remotePort, tls, summary)
	if err != nil {
		s.Logger.Error("create interaction failed", zap.Error(err))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	headers := make(map[string][]string)
	for k, v := range r.Header {
		headers[k] = v
	}
	headersJSON, _ := json.Marshal(headers)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.Logger.Warn("read body failed", zap.Error(err))
		body = nil
	}

	err = db.CreateHTTPInteraction(s.DB, interactionID, r.Method, scheme, r.Host, r.URL.Path, r.URL.RawQuery, r.Proto, string(headersJSON), body)
	if err != nil {
		s.Logger.Error("create http interaction failed", zap.Error(err))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
