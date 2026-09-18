package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/hebernet/hebernet/internal/protocol"
	"github.com/hebernet/hebernet/internal/store"
)

type provisionRequest struct {
	Domain    string
	AppType   string
	OwnerID   string
	PackageID string
	SkipLimits bool // paid Stripe orders: payment is the gate
}

type provisionResult struct {
	Site         *store.Site
	SFTPPassword string
	DBPassword   string
	AlreadyExists bool
}

func (s *Server) provisionSite(req provisionRequest) (*provisionResult, error) {
	req.Domain = strings.ToLower(strings.TrimSpace(req.Domain))
	if !domainRe.MatchString(req.Domain) {
		return nil, fmt.Errorf("domaine invalide")
	}
	if req.AppType != "wordpress" && req.AppType != "php" && req.AppType != "static" &&
		req.AppType != "laravel" && req.AppType != "prestashop" {
		return nil, fmt.Errorf("app_type: wordpress|php|static|laravel|prestashop")
	}

	if existing, err := s.Store.GetSiteByDomain(req.Domain); err == nil {
		if existing.Status == "active" || existing.Status == "provisioning" {
			return &provisionResult{Site: existing, AlreadyExists: true}, nil
		}
		// Site en erreur / suspendu : on repart proprement.
		_, _ = s.Agent.Call(protocol.OpDestroySite, map[string]any{
			"linux_user": existing.LinuxUser,
			"domain":     existing.Domain,
			"db_name":    existing.DBName,
			"db_user":    existing.DBUser,
		})
		_ = s.Store.DeleteSite(existing.ID)
	}

	owner, err := s.Store.GetUserByID(req.OwnerID)
	if err != nil || owner.Role != "client" {
		return nil, fmt.Errorf("owner_id client requis")
	}
	pkg, err := s.Store.GetPackage(req.PackageID)
	if err != nil {
		return nil, fmt.Errorf("package inconnu")
	}

	if !req.SkipLimits {
		siteCount, err := s.Store.CountOwnerSites(owner.ID)
		if err != nil {
			return nil, err
		}
		if siteCount >= pkg.MaxSites {
			return nil, fmt.Errorf("limite de sites atteinte pour ce package (%d)", pkg.MaxSites)
		}
	}

	needsDB := req.AppType == "wordpress" || req.AppType == "php" || req.AppType == "laravel" || req.AppType == "prestashop"
	if needsDB && !req.SkipLimits {
		dbCount, err := s.Store.CountOwnerDatabases(owner.ID)
		if err != nil {
			return nil, err
		}
		if dbCount >= pkg.MaxDatabases {
			return nil, fmt.Errorf("limite de bases de données atteinte pour ce package (%d)", pkg.MaxDatabases)
		}
	}

	linuxUser, err := s.uniqueLinuxUser(req.Domain)
	if err != nil {
		return nil, err
	}

	siteID := uuid.NewString()
	now := store.Now()
	site := store.Site{
		ID: siteID, Domain: req.Domain, AppType: req.AppType, OwnerID: owner.ID,
		PackageID: pkg.ID, LinuxUser: linuxUser, HomePath: "", PHPVersion: "8.5",
		Status: "provisioning", QuotaMB: pkg.DiskMB, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Store.CreateSite(site); err != nil {
		return nil, err
	}

	creds, err := s.Agent.Call(protocol.OpCreateSiteUser, map[string]any{"linux_user": linuxUser})
	if err != nil {
		site.Status = "error"
		_ = s.Store.UpdateSite(site)
		return nil, err
	}
	site.HomePath, _ = creds["home_path"].(string)
	site.SFTPUser, _ = creds["sftp_user"].(string)
	sftpPass, _ := creds["sftp_password"].(string)

	if _, err := s.Agent.Call(protocol.OpSetQuota, map[string]any{"linux_user": linuxUser, "quota_mb": pkg.DiskMB}); err != nil {
		// Quotas Linux optionnels (souvent absents / cassés sur VM lab).
		// Le site reste provisionné ; le plafond reste enregistré en base.
		log.Printf("provision %s: set_quota ignoré: %v", req.Domain, err)
	}

	if req.AppType != "static" {
		if _, err := s.Agent.Call(protocol.OpProvisionPHP, map[string]any{"linux_user": linuxUser, "php_version": "8.5"}); err != nil {
			site.Status = "error"
			_ = s.Store.UpdateSite(site)
			return nil, err
		}
	}
	if _, err := s.Agent.Call(protocol.OpProvisionNginx, map[string]any{
		"domain": req.Domain, "linux_user": linuxUser, "app_type": req.AppType,
	}); err != nil {
		site.Status = "error"
		_ = s.Store.UpdateSite(site)
		return nil, err
	}

	dbName := "hb_" + strings.ReplaceAll(linuxUser, "hb_", "")
	if len(dbName) > 32 {
		dbName = dbName[:32]
	}
	dbUser := dbName
	dbPass := ""
	if needsDB {
		dbRes, err := s.Agent.Call(protocol.OpCreateDatabase, map[string]any{"db_name": dbName, "db_user": dbUser})
		if err != nil {
			site.Status = "error"
			_ = s.Store.UpdateSite(site)
			return nil, err
		}
		site.DBName, _ = dbRes["db_name"].(string)
		site.DBUser, _ = dbRes["db_user"].(string)
		dbPass, _ = dbRes["db_password"].(string)
	}
	switch req.AppType {
	case "wordpress":
		if _, err := s.Agent.Call(protocol.OpProvisionWP, map[string]any{"linux_user": linuxUser}); err != nil {
			site.Status = "error"
			_ = s.Store.UpdateSite(site)
			return nil, err
		}
	case "laravel":
		if _, err := s.Agent.Call(protocol.OpProvisionLaravel, map[string]any{
			"linux_user": linuxUser,
			"db_name":    site.DBName,
			"db_user":    site.DBUser,
			"db_password": dbPass,
		}); err != nil {
			site.Status = "error"
			_ = s.Store.UpdateSite(site)
			return nil, err
		}
	case "prestashop":
		if _, err := s.Agent.Call(protocol.OpProvisionPrestaShop, map[string]any{"linux_user": linuxUser}); err != nil {
			site.Status = "error"
			_ = s.Store.UpdateSite(site)
			return nil, err
		}
	}

	site.Status = "active"
	site.UpdatedAt = store.Now()
	_ = s.Store.UpdateSite(site)
	full, _ := s.Store.GetSite(site.ID)
	return &provisionResult{Site: full, SFTPPassword: sftpPass, DBPassword: dbPass}, nil
}

func (s *Server) uniqueLinuxUser(domain string) (string, error) {
	base := "hb_" + strings.ReplaceAll(strings.Split(domain, ".")[0], "-", "_")
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, strings.ToLower(base))
	if len(base) > 24 {
		base = base[:24]
	}
	candidate := base
	for i := 0; i < 8; i++ {
		taken, err := s.Store.LinuxUserTaken(candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		suf := randomHex(3)
		candidate = base
		if len(candidate) > 24 {
			candidate = candidate[:24]
		}
		candidate = candidate + "_" + suf
		if len(candidate) > 32 {
			candidate = candidate[:32]
		}
	}
	return "", fmt.Errorf("impossible de générer un linux_user unique")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
