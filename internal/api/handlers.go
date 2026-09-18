package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hebernet/hebernet/internal/protocol"
	"github.com/hebernet/hebernet/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "hebernet-api"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "json invalide")
		return
	}
	u, err := s.Store.GetUserByEmail(strings.TrimSpace(strings.ToLower(body.Email)))
	if err != nil || !CheckPassword(u.PasswordHash, body.Password) {
		writeErr(w, http.StatusUnauthorized, "identifiants incorrects")
		return
	}
	token := randomToken()
	expires := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	if err := s.Store.CreateSession(token, u.ID, expires); err != nil {
		writeErr(w, http.StatusInternalServerError, "session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id": u.ID, "email": u.Email, "role": u.Role, "display_name": u.DisplayName,
		},
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	h := r.Header.Get("Authorization")
	token := strings.TrimPrefix(h, "Bearer ")
	_ = s.Store.DeleteSession(token)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleListPackages(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListPackages()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreatePackage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		DiskMB       int    `json:"disk_mb"`
		MaxSites     int    `json:"max_sites"`
		MaxDatabases int    `json:"max_databases"`
		Description  string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.DiskMB <= 0 {
		writeErr(w, http.StatusBadRequest, "name et disk_mb requis")
		return
	}
	if body.MaxSites <= 0 {
		body.MaxSites = 1
	}
	if body.MaxDatabases < 0 {
		body.MaxDatabases = 0
	}
	if body.MaxDatabases == 0 && body.MaxSites > 0 {
		// défaut raisonnable : autant de BDD que de sites
		body.MaxDatabases = body.MaxSites
	}
	p := store.Package{
		ID: uuid.NewString(), Name: body.Name, DiskMB: body.DiskMB,
		MaxSites: body.MaxSites, MaxDatabases: body.MaxDatabases,
		Description: body.Description, CreatedAt: store.Now(),
	}
	if err := s.Store.CreatePackage(p); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdatePackage(w http.ResponseWriter, r *http.Request) {
	existing, err := s.Store.GetPackage(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "package introuvable")
		return
	}
	var body struct {
		Name         string `json:"name"`
		DiskMB       int    `json:"disk_mb"`
		MaxSites     int    `json:"max_sites"`
		MaxDatabases int    `json:"max_databases"`
		Description  string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.DiskMB <= 0 {
		writeErr(w, http.StatusBadRequest, "name et disk_mb requis")
		return
	}
	if body.MaxSites <= 0 {
		body.MaxSites = 1
	}
	if body.MaxDatabases < 0 {
		body.MaxDatabases = 0
	}
	existing.Name = body.Name
	existing.DiskMB = body.DiskMB
	existing.MaxSites = body.MaxSites
	existing.MaxDatabases = body.MaxDatabases
	existing.Description = body.Description
	if err := s.Store.UpdatePackage(*existing); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeletePackage(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeletePackage(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "json invalide")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		writeErr(w, http.StatusBadRequest, "email et password requis")
		return
	}
	if body.Role != "admin" && body.Role != "client" {
		body.Role = "client"
	}
	hash, err := HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash")
		return
	}
	u := store.User{
		ID: uuid.NewString(), Email: body.Email, PasswordHash: hash,
		Role: body.Role, DisplayName: body.DisplayName, CreatedAt: store.Now(),
	}
	if err := s.Store.CreateUser(u); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var (
		list []store.Site
		err  error
	)
	if u.Role == "admin" {
		list, err = s.Store.ListSites()
	} else {
		list, err = s.Store.ListSitesByOwner(u.ID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) canAccessSite(u *store.User, site *store.Site) bool {
	return u.Role == "admin" || site.OwnerID == u.ID
}

func (s *Server) handleGetSite(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "site introuvable")
		return
	}
	if !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	writeJSON(w, http.StatusOK, site)
}

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

func (s *Server) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain    string `json:"domain"`
		AppType   string `json:"app_type"`
		OwnerID   string `json:"owner_id"`
		PackageID string `json:"package_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "json invalide")
		return
	}
	res, err := s.provisionSite(provisionRequest{
		Domain: body.Domain, AppType: body.AppType, OwnerID: body.OwnerID, PackageID: body.PackageID,
	})
	if err != nil {
		msg := err.Error()
		status := http.StatusBadGateway
		if strings.Contains(msg, "domaine invalide") || strings.Contains(msg, "app_type") ||
			strings.Contains(msg, "owner_id") || strings.Contains(msg, "package") {
			status = http.StatusBadRequest
		}
		if strings.Contains(msg, "limite") || strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "unique") {
			status = http.StatusConflict
		}
		writeErr(w, status, msg)
		return
	}
	if res.AlreadyExists {
		writeJSON(w, http.StatusOK, map[string]any{"site": res.Site, "note": "Site déjà existant."})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"site":          res.Site,
		"sftp_password": res.SFTPPassword,
		"note":          "Mot de passe SFTP affiché une seule fois.",
	})
}

func (s *Server) handleSiteSSL(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	if _, err := s.Agent.Call(protocol.OpIssueSSL, map[string]any{"domain": site.Domain}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	site.SSLEnabled = true
	site.UpdatedAt = store.Now()
	_ = s.Store.UpdateSite(*site)
	writeJSON(w, http.StatusOK, site)
}

func (s *Server) handleSiteSFTPReset(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	res, err := s.Agent.Call(protocol.OpResetSFTPPass, map[string]any{"linux_user": site.LinuxUser})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sftp_user": site.SFTPUser, "sftp_password": res["sftp_password"]})
}

func (s *Server) handleSiteSuspend(w http.ResponseWriter, r *http.Request) {
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "site introuvable")
		return
	}
	if _, err := s.Agent.Call(protocol.OpSuspendSite, map[string]any{"domain": site.Domain}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	site.Status = "suspended"
	site.UpdatedAt = store.Now()
	_ = s.Store.UpdateSite(*site)
	writeJSON(w, http.StatusOK, site)
}

func (s *Server) handleSiteResume(w http.ResponseWriter, r *http.Request) {
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "site introuvable")
		return
	}
	if _, err := s.Agent.Call(protocol.OpResumeSite, map[string]any{"domain": site.Domain}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	site.Status = "active"
	site.UpdatedAt = store.Now()
	_ = s.Store.UpdateSite(*site)
	writeJSON(w, http.StatusOK, site)
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "site introuvable")
		return
	}
	if _, err := s.Agent.Call(protocol.OpDestroySite, map[string]any{
		"linux_user": site.LinuxUser,
		"domain":     site.Domain,
		"db_name":    site.DBName,
		"db_user":    site.DBUser,
	}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	_ = s.Store.DeleteSite(site.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Site, fichiers, config et base (si présente) supprimés.",
	})
}

func (s *Server) handleSiteQuota(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	res, err := s.Agent.Call(protocol.OpQuotaUsage, map[string]any{"linux_user": site.LinuxUser})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	used := 0
	if v, ok := res["used_mb"].(float64); ok {
		used = int(v)
	}
	site.QuotaUsedMB = used
	site.UpdatedAt = store.Now()
	_ = s.Store.UpdateSite(*site)
	writeJSON(w, http.StatusOK, map[string]any{
		"quota_mb": site.QuotaMB, "used_mb": used,
		"percent": func() float64 {
			if site.QuotaMB == 0 {
				return 0
			}
			return float64(used) / float64(site.QuotaMB) * 100
		}(),
	})
}

func (s *Server) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "access"
	}
	res, err := s.Agent.Call(protocol.OpReadLogs, map[string]any{"domain": site.Domain, "kind": kind})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSiteDBReset(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	if site.DBUser == "" {
		writeErr(w, http.StatusBadRequest, "pas de base sur ce site")
		return
	}
	res, err := s.Agent.Call(protocol.OpResetDBPass, map[string]any{"db_user": site.DBUser})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"db_name": site.DBName, "db_user": site.DBUser, "db_password": res["db_password"],
	})
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	ping, err := s.Agent.Call(protocol.OpPing, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_ok": err == nil,
		"agent":    ping,
		"php":      "8.5",
		"stack":    []string{"nginx", "php-fpm", "mariadb", "certbot"},
	})
}
