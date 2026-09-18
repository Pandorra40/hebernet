package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/hebernet/hebernet/internal/agentclient"
	"github.com/hebernet/hebernet/internal/mail"
	"github.com/hebernet/hebernet/internal/store"
	"github.com/hebernet/hebernet/internal/tools"
)

type Server struct {
	Store *store.Store
	Agent *agentclient.Client
	Tools *tools.Manager
	Mux   *http.ServeMux
	CORS  string

	StripeWebhookSecret string
	StripeSecretKey     string
	StripePackageName   string
	StripePriceCents    int
	StripeCheckoutDemo  bool
	VitrineURL          string
	// PublicHost: hostname/IP joignable depuis le navigateur (VM), pour FileBrowser / Adminer.
	PublicHost string
	Mail                mail.Config
}

type ctxKey int

const userKey ctxKey = 1

func New(st *store.Store, ag *agentclient.Client, tm *tools.Manager) *Server {
	s := &Server{Store: st, Agent: ag, Tools: tm, Mux: http.NewServeMux(), CORS: "*"}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.CORS)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Stripe-Signature")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.Mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.Mux.HandleFunc("GET /api/health", s.handleHealth)
	s.Mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.Mux.HandleFunc("POST /api/auth/logout", s.auth(s.handleLogout))
	s.Mux.HandleFunc("GET /api/auth/me", s.auth(s.handleMe))

	s.Mux.HandleFunc("GET /api/packages", s.auth(s.handleListPackages))
	s.Mux.HandleFunc("POST /api/packages", s.authRole("admin", s.handleCreatePackage))
	s.Mux.HandleFunc("PUT /api/packages/{id}", s.authRole("admin", s.handleUpdatePackage))
	s.Mux.HandleFunc("DELETE /api/packages/{id}", s.authRole("admin", s.handleDeletePackage))

	s.Mux.HandleFunc("GET /api/users", s.authRole("admin", s.handleListUsers))
	s.Mux.HandleFunc("POST /api/users", s.authRole("admin", s.handleCreateUser))

	s.Mux.HandleFunc("GET /api/sites", s.auth(s.handleListSites))
	s.Mux.HandleFunc("POST /api/sites", s.authRole("admin", s.handleCreateSite))
	s.Mux.HandleFunc("GET /api/sites/{id}", s.auth(s.handleGetSite))
	s.Mux.HandleFunc("POST /api/sites/{id}/ssl", s.auth(s.handleSiteSSL))
	s.Mux.HandleFunc("POST /api/sites/{id}/sftp-reset", s.auth(s.handleSiteSFTPReset))
	s.Mux.HandleFunc("POST /api/sites/{id}/suspend", s.authRole("admin", s.handleSiteSuspend))
	s.Mux.HandleFunc("POST /api/sites/{id}/resume", s.authRole("admin", s.handleSiteResume))
	s.Mux.HandleFunc("DELETE /api/sites/{id}", s.authRole("admin", s.handleDeleteSite))
	s.Mux.HandleFunc("GET /api/sites/{id}/quota", s.auth(s.handleSiteQuota))
	s.Mux.HandleFunc("GET /api/sites/{id}/logs", s.auth(s.handleSiteLogs))
	s.Mux.HandleFunc("POST /api/sites/{id}/db-reset", s.auth(s.handleSiteDBReset))
	s.Mux.HandleFunc("GET /api/sites/{id}/cron", s.auth(s.handleListCron))
	s.Mux.HandleFunc("POST /api/sites/{id}/cron", s.auth(s.handleCreateCron))
	s.Mux.HandleFunc("DELETE /api/sites/{id}/cron/{jobId}", s.auth(s.handleDeleteCron))
	s.Mux.HandleFunc("POST /api/sites/{id}/adminer", s.auth(s.handleOpenAdminer))
	s.Mux.HandleFunc("POST /api/sites/{id}/files", s.auth(s.handleOpenFiles))

	s.Mux.HandleFunc("GET /api/server", s.authRole("admin", s.handleServerInfo))

	s.Mux.HandleFunc("GET /api/stripe/webhook", s.handleStripeWebhook)
	s.Mux.HandleFunc("POST /api/stripe/webhook", s.handleStripeWebhook)
	s.Mux.HandleFunc("POST /api/stripe/checkout", s.handleStripeCheckout)
	s.Mux.HandleFunc("GET /api/stripe/status/{id}", s.handleStripeStatus)
	s.Mux.HandleFunc("GET /api/stripe/status", s.handleStripeStatus)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// sidecarURL builds a browser-reachable URL for FileBrowser / Adminer.
func (s *Server) sidecarURL(port int, path string) string {
	host := strings.TrimSpace(s.PublicHost)
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d%s", host, port, path)
}

// PublicHostFromURL extracts host from HEBERNET_PANEL_URL (sans port si :80).
func PublicHostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "127.0.0.1"
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "127.0.0.1"
	}
	return u.Hostname()
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.userFromRequest(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "non authentifié")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	}
}

func (s *Server) authRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return s.auth(func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r)
		if u == nil || u.Role != role {
			writeErr(w, http.StatusForbidden, "accès refusé")
			return
		}
		next(w, r)
	})
}

func (s *Server) userFromRequest(r *http.Request) (*store.User, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, errors.New("missing token")
	}
	token := strings.TrimPrefix(h, "Bearer ")
	return s.Store.GetSessionUser(token)
}

func userFrom(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
