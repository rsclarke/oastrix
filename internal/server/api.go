// Package server implements the HTTP, HTTPS, DNS, and API servers.
package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rsclarke/oastrix/internal/api"
	"github.com/rsclarke/oastrix/internal/auth"
	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/token"
	"go.uber.org/zap"
)

type contextKey string

const apiKeyIDContextKey contextKey = "apiKeyID"

func getAPIKeyID(r *http.Request) int64 {
	if id, ok := r.Context().Value(apiKeyIDContextKey).(int64); ok {
		return id
	}
	return 0
}

// APIServer handles the REST API for token and interaction management.
type APIServer struct {
	DB       *sql.DB
	Domain   string
	Logger   *zap.Logger
	PublicIP string
}

// AuthMiddleware validates API key authentication for protected routes.
func (s *APIServer) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		apiKey := strings.TrimPrefix(authHeader, "Bearer ")

		prefix, _, err := auth.ParseAPIKey(apiKey)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		storedKey, err := db.GetAPIKeyByPrefix(s.DB, prefix)
		if err != nil || storedKey == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if storedKey.RevokedAt != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if !auth.VerifyAPIKey(apiKey, storedKey.KeyHash) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		ctx := context.WithValue(r.Context(), apiKeyIDContextKey, storedKey.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Handler returns the HTTP handler for the API server.
func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tokens", s.handleCreateToken)
	mux.HandleFunc("GET /v1/tokens", s.handleListTokens)
	mux.HandleFunc("GET /v1/tokens/{token}/interactions", s.handleGetInteractions)
	mux.HandleFunc("DELETE /v1/tokens/{token}", s.handleDeleteToken)

	return s.AuthMiddleware(mux)
}

func (s *APIServer) handleListTokens(w http.ResponseWriter, r *http.Request) {
	apiKeyID := getAPIKeyID(r)
	tokens, err := db.ListTokensByAPIKey(s.DB, apiKeyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	resp := api.ListTokensResponse{
		Tokens: make([]api.TokenInfo, 0, len(tokens)),
	}
	for _, t := range tokens {
		resp.Tokens = append(resp.Tokens, api.TokenInfo{
			Token:            t.Token,
			Label:            t.Label,
			CreatedAt:        time.Unix(t.CreatedAt, 0).UTC().Format(time.RFC3339),
			InteractionCount: t.InteractionCount,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req api.CreateTokenRequest
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16) // 64KB limit
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil && err != io.EOF {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		// Ensure no trailing data
		if dec.Decode(&struct{}{}) != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unexpected trailing data"})
			return
		}
	}

	tok, err := token.Generate()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	var labelPtr *string
	if req.Label != "" {
		labelPtr = &req.Label
	}

	// Associate token with the API key that created it
	apiKeyID := getAPIKeyID(r)
	_, err = db.CreateToken(s.DB, tok, &apiKeyID, labelPtr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}

	resp := api.CreateTokenResponse{
		Token: tok,
		Payloads: map[string]string{
			"dns":   fmt.Sprintf("%s.%s", tok, s.Domain),
			"http":  fmt.Sprintf("http://%s.%s/", tok, s.Domain),
			"https": fmt.Sprintf("https://%s.%s/", tok, s.Domain),
		},
	}

	if s.PublicIP != "" {
		resp.Payloads["http_ip"] = fmt.Sprintf("http://%s/oast/%s", s.PublicIP, tok)
		resp.Payloads["https_ip"] = fmt.Sprintf("https://%s/oast/%s", s.PublicIP, tok)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleGetInteractions(w http.ResponseWriter, r *http.Request) {
	tokenValue := r.PathValue("token")
	if tokenValue == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}

	tok, err := db.GetTokenByValue(s.DB, tokenValue)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if tok == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
		return
	}

	// Verify ownership: token must belong to the requesting API key
	apiKeyID := getAPIKeyID(r)
	if tok.APIKeyID == nil || *tok.APIKeyID != apiKeyID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
		return
	}

	interactions, err := db.GetInteractionsByToken(s.DB, tok.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	resp := api.GetInteractionsResponse{
		Token:        tokenValue,
		Interactions: make([]api.InteractionResponse, 0, len(interactions)),
	}

	for _, i := range interactions {
		ir := api.InteractionResponse{
			ID:         i.ID,
			Kind:       i.Kind,
			OccurredAt: time.Unix(i.OccurredAt, 0).UTC().Format(time.RFC3339),
			RemoteIP:   i.RemoteIP,
			RemotePort: i.RemotePort,
			TLS:        i.TLS,
			Summary:    i.Summary,
		}

		if i.Kind == "http" {
			httpInt, err := db.GetHTTPInteraction(s.DB, i.ID)
			if err != nil {
				s.Logger.Error("failed to get HTTP interaction details",
					zap.Int64("interaction_id", i.ID),
					zap.Error(err))
			} else if httpInt != nil {
				var headers map[string][]string
				if err := json.Unmarshal([]byte(httpInt.RequestHeaders), &headers); err != nil {
					s.Logger.Warn("failed to parse stored request headers",
						zap.Int64("interaction_id", i.ID),
						zap.Error(err))
					headers = make(map[string][]string)
				}

				ir.HTTP = &api.HTTPInteractionDetail{
					Method:  httpInt.Method,
					Scheme:  httpInt.Scheme,
					Host:    httpInt.Host,
					Path:    httpInt.Path,
					Query:   httpInt.Query,
					Headers: headers,
					Body:    base64.StdEncoding.EncodeToString(httpInt.RequestBody),
				}
			}
		}

		if i.Kind == "dns" {
			dnsInt, err := db.GetDNSInteraction(s.DB, i.ID)
			if err != nil {
				s.Logger.Error("failed to get DNS interaction details",
					zap.Int64("interaction_id", i.ID),
					zap.Error(err))
			} else if dnsInt != nil {
				ir.DNS = &api.DNSInteractionDetail{
					QName:    dnsInt.QName,
					QType:    dnsInt.QType,
					QClass:   dnsInt.QClass,
					RD:       dnsInt.RD != 0,
					Opcode:   dnsInt.Opcode,
					DNSID:    dnsInt.DNSID,
					Protocol: dnsInt.Protocol,
				}
			}
		}

		resp.Interactions = append(resp.Interactions, ir)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	tokenValue := r.PathValue("token")
	if tokenValue == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}

	tok, err := db.GetTokenByValue(s.DB, tokenValue)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if tok == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
		return
	}

	// Verify ownership: token must belong to the requesting API key
	apiKeyID := getAPIKeyID(r)
	if tok.APIKeyID == nil || *tok.APIKeyID != apiKeyID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
		return
	}

	err = db.DeleteToken(s.DB, tokenValue)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete token"})
		return
	}

	writeJSON(w, http.StatusOK, api.DeleteTokenResponse{Deleted: true})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}
