package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hebernet/hebernet/internal/protocol"
	"github.com/hebernet/hebernet/internal/store"
)

func (s *Server) handleListCron(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	list, err := s.Store.ListCronJobs(site.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateCron(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	var body struct {
		Schedule string `json:"schedule"`
		Command  string `json:"command"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "json invalide")
		return
	}
	body.Schedule = strings.TrimSpace(body.Schedule)
	body.Command = strings.TrimSpace(body.Command)
	if body.Schedule == "" || body.Command == "" {
		writeErr(w, http.StatusBadRequest, "schedule et command requis")
		return
	}
	if strings.ContainsAny(body.Command, "\n\r") {
		writeErr(w, http.StatusBadRequest, "commande invalide")
		return
	}
	en := true
	if body.Enabled != nil {
		en = *body.Enabled
	}
	job := store.CronJob{
		ID: uuid.NewString(), SiteID: site.ID, Schedule: body.Schedule,
		Command: body.Command, Enabled: en, CreatedAt: store.Now(),
	}
	if err := s.Store.CreateCronJob(job); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.syncSiteCron(site); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) handleDeleteCron(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	if err := s.Store.DeleteCronJob(r.PathValue("jobId"), site.ID); err != nil {
		writeErr(w, http.StatusNotFound, "tâche introuvable")
		return
	}
	if err := s.syncSiteCron(site); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) syncSiteCron(site *store.Site) error {
	jobs, err := s.Store.ListCronJobs(site.ID)
	if err != nil {
		return err
	}
	payload := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		payload = append(payload, map[string]any{
			"schedule": j.Schedule, "command": j.Command, "enabled": j.Enabled,
		})
	}
	_, err = s.Agent.Call(protocol.OpSyncCron, map[string]any{
		"linux_user": site.LinuxUser, "jobs": payload,
	})
	return err
}

func (s *Server) handleOpenAdminer(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	if site.DBName == "" {
		writeErr(w, http.StatusBadRequest, "pas de base sur ce site")
		return
	}
	if s.Tools == nil {
		writeErr(w, http.StatusServiceUnavailable, "tools non configurés")
		return
	}
	if err := s.Tools.EnsureAdminerPHP(); err != nil {
		writeErr(w, http.StatusBadGateway, "téléchargement Adminer: "+err.Error())
		return
	}
	_, err = s.Agent.Call(protocol.OpInstallAdminer, map[string]any{
		"linux_user":  site.LinuxUser,
		"adminer_src": s.Tools.AdminerPHP,
		"adminer_css": s.Tools.AdminerCSS,
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	home := site.HomePath
	if home == "" {
		writeErr(w, http.StatusInternalServerError, "home manquant")
		return
	}
	docRoot := filepath.Join(home, "public_html")
	port, err := s.Tools.StartPHP(docRoot)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "php -S: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":      s.sidecarURL(port, "/.hebernet/adminer.php"),
		"db_name":  site.DBName,
		"db_user":  site.DBUser,
		"db_host":  "localhost",
		"note":     "En démo sans MariaDB, Adminer s’ouvre mais la connexion SQL peut échouer. Sur un hôte réel, utilisez les credentials du site.",
	})
}

func (s *Server) handleOpenFiles(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	site, err := s.Store.GetSite(r.PathValue("id"))
	if err != nil || !s.canAccessSite(u, site) {
		writeErr(w, http.StatusForbidden, "accès refusé")
		return
	}
	if s.Tools == nil {
		writeErr(w, http.StatusServiceUnavailable, "tools non configurés")
		return
	}

	// Re-assert package disk quota (Linux setquota = source of truth when available).
	if site.QuotaMB > 0 {
		if _, err := s.Agent.Call(protocol.OpSetQuota, map[string]any{
			"linux_user": site.LinuxUser, "quota_mb": site.QuotaMB,
		}); err != nil {
			log.Printf("site %s set_quota: %v", site.Domain, err)
		}
	}

	res, err := s.Agent.Call(protocol.OpEnsureFM, map[string]any{"linux_user": site.LinuxUser})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	root, _ := res["root"].(string)
	if root == "" {
		root = site.HomePath
	}
	if err := s.Tools.EnsureFilebrowserBin(); err != nil {
		writeErr(w, http.StatusBadGateway, "téléchargement FileBrowser: "+err.Error())
		return
	}
	port, err := s.Tools.StartFilebrowser(root)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	used := site.QuotaUsedMB
	if qres, err := s.Agent.Call(protocol.OpQuotaUsage, map[string]any{"linux_user": site.LinuxUser}); err == nil {
		if v, ok := qres["used_mb"].(float64); ok {
			used = int(v)
			site.QuotaUsedMB = used
			site.UpdatedAt = store.Now()
			_ = s.Store.UpdateSite(*site)
		}
	}

	limitBytes := int64(site.QuotaMB) * 1024 * 1024
	qm, _ := s.Tools.SyncQuantumQuota(limitBytes)
	pct := float64(0)
	if site.QuotaMB > 0 {
		pct = float64(used) / float64(site.QuotaMB) * 100
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"url":        s.sidecarURL(port, ""),
		"root":       root,
		"quota_mb":   site.QuotaMB,
		"used_mb":    used,
		"percent":    pct,
		"limit_bytes": limitBytes,
		"quantum":    qm,
		"note":       fmt.Sprintf("Quota site Hébernet : %d / %d Mo (setquota). Quantum v2.0.6 n’expose pas encore limitBytes.", used, site.QuotaMB),
	})
}
