package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rsclarke/oastrix/internal/auth"
	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/token"
)

type contextKey string

const apiKeyIDContextKey contextKey = "apiKeyID"

func getAPIKeyID(r *http.Request) int64 {
	if id, ok := r.Context().Value(apiKeyIDContextKey).(int64); ok {
		return id
	}
	return 0
}

type APIServer struct {
	DB     *sql.DB
	Domain string
	Pepper []byte
}

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

		if !auth.VerifyAPIKey(apiKey, storedKey.KeyHash, s.Pepper) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		// Add API key ID to context for ownership checks
		ctx := context.WithValue(r.Context(), apiKeyIDContextKey, storedKey.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tokens", s.handleCreateToken)
	mux.HandleFunc("GET /v1/tokens", s.handleListTokens)
	mux.HandleFunc("GET /v1/tokens/{token}/interactions", s.handleGetInteractions)
	mux.HandleFunc("DELETE /v1/tokens/{token}", s.handleDeleteToken)

	return s.AuthMiddleware(mux)
}

type createTokenRequest struct {
	Label string `json:"label"`
}

type createTokenResponse struct {
	Token    string            `json:"token"`
	Payloads map[string]string `json:"payloads"`
}

type listTokensResponse struct {
	Tokens []tokenInfo `json:"tokens"`
}

type tokenInfo struct {
	Token            string  `json:"token"`
	Label            *string `json:"label"`
	CreatedAt        string  `json:"created_at"`
	InteractionCount int     `json:"interaction_count"`
}

func (s *APIServer) handleListTokens(w http.ResponseWriter, r *http.Request) {
	apiKeyID := getAPIKeyID(r)
	tokens, err := db.ListTokensByAPIKey(s.DB, apiKeyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	resp := listTokensResponse{
		Tokens: make([]tokenInfo, 0, len(tokens)),
	}
	for _, t := range tokens {
		resp.Tokens = append(resp.Tokens, tokenInfo{
			Token:            t.Token,
			Label:            t.Label,
			CreatedAt:        time.Unix(t.CreatedAt, 0).UTC().Format(time.RFC3339),
			InteractionCount: t.InteractionCount,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	tok, err := token.Generate()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	existing, err := db.GetTokenByValue(s.DB, tok)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token collision, please retry"})
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

	resp := createTokenResponse{
		Token: tok,
		Payloads: map[string]string{
			"http":  fmt.Sprintf("http://%s.%s/", tok, s.Domain),
			"https": fmt.Sprintf("https://%s.%s/", tok, s.Domain),
			"dns":   fmt.Sprintf("%s.%s", tok, s.Domain),
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

type interactionResponse struct {
	ID         int64                 `json:"id"`
	Kind       string                `json:"kind"`
	OccurredAt string                `json:"occurred_at"`
	RemoteIP   string                `json:"remote_ip"`
	RemotePort int                   `json:"remote_port"`
	TLS        bool                  `json:"tls"`
	Summary    string                `json:"summary"`
	HTTP       *httpInteractionDetail `json:"http,omitempty"`
	DNS        *dnsInteractionDetail  `json:"dns,omitempty"`
}

type httpInteractionDetail struct {
	Method  string              `json:"method"`
	Scheme  string              `json:"scheme"`
	Host    string              `json:"host"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type dnsInteractionDetail struct {
	QName    string `json:"qname"`
	QType    int    `json:"qtype"`
	QClass   int    `json:"qclass"`
	RD       bool   `json:"rd"`
	Opcode   int    `json:"opcode"`
	DNSID    int    `json:"dns_id"`
	Protocol string `json:"protocol"`
}

type getInteractionsResponse struct {
	Token        string                `json:"token"`
	Interactions []interactionResponse `json:"interactions"`
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

	resp := getInteractionsResponse{
		Token:        tokenValue,
		Interactions: make([]interactionResponse, 0, len(interactions)),
	}

	for _, i := range interactions {
		ir := interactionResponse{
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
			if err == nil && httpInt != nil {
				var headers map[string][]string
				json.Unmarshal([]byte(httpInt.RequestHeaders), &headers)

				ir.HTTP = &httpInteractionDetail{
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
			if err == nil && dnsInt != nil {
				ir.DNS = &dnsInteractionDetail{
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

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
