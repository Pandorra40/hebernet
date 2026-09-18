package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	DisplayName  string `json:"display_name"`
	CreatedAt    string `json:"created_at"`
}

type Package struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DiskMB       int    `json:"disk_mb"`
	MaxSites     int    `json:"max_sites"`
	MaxDatabases int    `json:"max_databases"`
	Description  string `json:"description"`
	CreatedAt    string `json:"created_at"`
}

type Site struct {
	ID          string `json:"id"`
	Domain      string `json:"domain"`
	AppType     string `json:"app_type"`
	OwnerID     string `json:"owner_id"`
	PackageID   string `json:"package_id"`
	LinuxUser   string `json:"linux_user"`
	HomePath    string `json:"home_path"`
	PHPVersion  string `json:"php_version"`
	Status      string `json:"status"`
	SSLEnabled  bool   `json:"ssl_enabled"`
	DBName      string `json:"db_name"`
	DBUser      string `json:"db_user"`
	SFTPUser    string `json:"sftp_user"`
	QuotaMB     int    `json:"quota_mb"`
	QuotaUsedMB int    `json:"quota_used_mb"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	OwnerEmail  string `json:"owner_email,omitempty"`
	PackageName string `json:"package_name,omitempty"`
}

func (s *Store) CreateUser(u User) error {
	_, err := s.DB.Exec(
		`INSERT INTO users (id, email, password_hash, role, display_name, created_at) VALUES (?,?,?,?,?,?)`,
		u.ID, u.Email, u.PasswordHash, u.Role, u.DisplayName, u.CreatedAt,
	)
	return err
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, email, password_hash, role, display_name, created_at FROM users WHERE email = ?`, email)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(id string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, email, password_hash, role, display_name, created_at FROM users WHERE id = ?`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.DB.Query(`SELECT id, email, password_hash, role, display_name, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CreatePackage(p Package) error {
	_, err := s.DB.Exec(
		`INSERT INTO packages (id, name, disk_mb, max_sites, max_databases, description, created_at) VALUES (?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.DiskMB, p.MaxSites, p.MaxDatabases, p.Description, p.CreatedAt,
	)
	return err
}

func (s *Store) ListPackages() ([]Package, error) {
	rows, err := s.DB.Query(`SELECT id, name, disk_mb, max_sites, COALESCE(max_databases,1), description, created_at FROM packages ORDER BY disk_mb`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.Name, &p.DiskMB, &p.MaxSites, &p.MaxDatabases, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPackage(id string) (*Package, error) {
	row := s.DB.QueryRow(`SELECT id, name, disk_mb, max_sites, COALESCE(max_databases,1), description, created_at FROM packages WHERE id = ?`, id)
	var p Package
	if err := row.Scan(&p.ID, &p.Name, &p.DiskMB, &p.MaxSites, &p.MaxDatabases, &p.Description, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) UpdatePackage(p Package) error {
	_, err := s.DB.Exec(
		`UPDATE packages SET name=?, disk_mb=?, max_sites=?, max_databases=?, description=? WHERE id=?`,
		p.Name, p.DiskMB, p.MaxSites, p.MaxDatabases, p.Description, p.ID,
	)
	return err
}

func (s *Store) DeletePackage(id string) error {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sites WHERE package_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("package utilisé par %d site(s)", n)
	}
	_, err := s.DB.Exec(`DELETE FROM packages WHERE id = ?`, id)
	return err
}

func (s *Store) GetPackageByName(name string) (*Package, error) {
	row := s.DB.QueryRow(
		`SELECT id, name, disk_mb, max_sites, COALESCE(max_databases,1), description, created_at FROM packages WHERE name = ?`,
		name,
	)
	var p Package
	if err := row.Scan(&p.ID, &p.Name, &p.DiskMB, &p.MaxSites, &p.MaxDatabases, &p.Description, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) GetSiteByDomain(domain string) (*Site, error) {
	row := s.DB.QueryRow(siteSelect+` WHERE s.domain = ?`, domain)
	site, err := scanSite(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return site, nil
}

func (s *Store) LinuxUserTaken(linuxUser string) (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM sites WHERE linux_user = ?`, linuxUser).Scan(&n)
	return n > 0, err
}

func (s *Store) StripeSessionProcessed(sessionID string) (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM stripe_sessions WHERE session_id = ?`, sessionID).Scan(&n)
	return n > 0, err
}

func (s *Store) MarkStripeSession(sessionID, domain, email, siteID string) error {
	_, err := s.DB.Exec(
		`INSERT INTO stripe_sessions (session_id, domain, email, site_id, processed_at) VALUES (?,?,?,?,?)
		 ON CONFLICT(session_id) DO UPDATE SET domain=excluded.domain, email=excluded.email, site_id=excluded.site_id, processed_at=excluded.processed_at`,
		sessionID, domain, email, siteID, Now(),
	)
	return err
}

func (s *Store) SaveStripePending(sessionID, domain, email, appType string) error {
	_, err := s.DB.Exec(
		`INSERT OR REPLACE INTO stripe_pending (session_id, domain, email, app_type, created_at) VALUES (?,?,?,?,?)`,
		sessionID, domain, email, appType, Now(),
	)
	return err
}

type StripePending struct {
	SessionID string
	Domain    string
	Email     string
	AppType   string
	CreatedAt string
}

func (s *Store) GetStripePending(sessionID string) (*StripePending, error) {
	row := s.DB.QueryRow(
		`SELECT session_id, domain, email, app_type, created_at FROM stripe_pending WHERE session_id = ?`,
		sessionID,
	)
	var p StripePending
	if err := row.Scan(&p.SessionID, &p.Domain, &p.Email, &p.AppType, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) GetStripeSession(sessionID string) (domain, email, siteID, processedAt string, err error) {
	err = s.DB.QueryRow(
		`SELECT domain, email, site_id, processed_at FROM stripe_sessions WHERE session_id = ?`,
		sessionID,
	).Scan(&domain, &email, &siteID, &processedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", "", ErrNotFound
	}
	return
}

func (s *Store) CountOwnerSites(ownerID string) (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM sites WHERE owner_id = ?`, ownerID).Scan(&n)
	return n, err
}

func (s *Store) CountOwnerDatabases(ownerID string) (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM sites WHERE owner_id = ? AND db_name != ''`, ownerID).Scan(&n)
	return n, err
}

func (s *Store) CreateSite(site Site) error {
	ssl := 0
	if site.SSLEnabled {
		ssl = 1
	}
	_, err := s.DB.Exec(`INSERT INTO sites (
		id, domain, app_type, owner_id, package_id, linux_user, home_path, php_version, status,
		ssl_enabled, db_name, db_user, sftp_user, quota_mb, quota_used_mb, created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		site.ID, site.Domain, site.AppType, site.OwnerID, site.PackageID, site.LinuxUser, site.HomePath,
		site.PHPVersion, site.Status, ssl, site.DBName, site.DBUser, site.SFTPUser, site.QuotaMB,
		site.QuotaUsedMB, site.CreatedAt, site.UpdatedAt,
	)
	return err
}

func scanSite(scanner interface{ Scan(dest ...any) error }) (*Site, error) {
	var site Site
	var ssl int
	if err := scanner.Scan(
		&site.ID, &site.Domain, &site.AppType, &site.OwnerID, &site.PackageID, &site.LinuxUser, &site.HomePath,
		&site.PHPVersion, &site.Status, &ssl, &site.DBName, &site.DBUser, &site.SFTPUser, &site.QuotaMB,
		&site.QuotaUsedMB, &site.CreatedAt, &site.UpdatedAt, &site.OwnerEmail, &site.PackageName,
	); err != nil {
		return nil, err
	}
	site.SSLEnabled = ssl == 1
	return &site, nil
}

const siteSelect = `SELECT s.id, s.domain, s.app_type, s.owner_id, s.package_id, s.linux_user, s.home_path,
	s.php_version, s.status, s.ssl_enabled, s.db_name, s.db_user, s.sftp_user, s.quota_mb, s.quota_used_mb,
	s.created_at, s.updated_at, u.email, p.name
	FROM sites s
	JOIN users u ON u.id = s.owner_id
	JOIN packages p ON p.id = s.package_id`

func (s *Store) GetSite(id string) (*Site, error) {
	row := s.DB.QueryRow(siteSelect+` WHERE s.id = ?`, id)
	site, err := scanSite(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return site, nil
}

func (s *Store) ListSites() ([]Site, error) {
	rows, err := s.DB.Query(siteSelect + ` ORDER BY s.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSites(rows)
}

func (s *Store) ListSitesByOwner(ownerID string) ([]Site, error) {
	rows, err := s.DB.Query(siteSelect+` WHERE s.owner_id = ? ORDER BY s.created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSites(rows)
}

func collectSites(rows *sql.Rows) ([]Site, error) {
	var out []Site
	for rows.Next() {
		site, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *site)
	}
	if out == nil {
		out = []Site{}
	}
	return out, rows.Err()
}

func (s *Store) UpdateSite(site Site) error {
	ssl := 0
	if site.SSLEnabled {
		ssl = 1
	}
	_, err := s.DB.Exec(`UPDATE sites SET domain=?, app_type=?, owner_id=?, package_id=?, linux_user=?, home_path=?,
		php_version=?, status=?, ssl_enabled=?, db_name=?, db_user=?, sftp_user=?, quota_mb=?, quota_used_mb=?, updated_at=?
		WHERE id=?`,
		site.Domain, site.AppType, site.OwnerID, site.PackageID, site.LinuxUser, site.HomePath, site.PHPVersion,
		site.Status, ssl, site.DBName, site.DBUser, site.SFTPUser, site.QuotaMB, site.QuotaUsedMB, site.UpdatedAt, site.ID,
	)
	return err
}

func (s *Store) DeleteSite(id string) error {
	_, err := s.DB.Exec(`DELETE FROM sites WHERE id = ?`, id)
	return err
}

func (s *Store) CreateSession(token, userID, expiresAt string) error {
	_, err := s.DB.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?,?,?)`, token, userID, expiresAt)
	return err
}

func (s *Store) GetSessionUser(token string) (*User, error) {
	row := s.DB.QueryRow(`SELECT u.id, u.email, u.password_hash, u.role, u.display_name, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > ?`, token, Now())
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

type CronJob struct {
	ID        string `json:"id"`
	SiteID    string `json:"site_id"`
	Schedule  string `json:"schedule"`
	Command   string `json:"command"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) ListCronJobs(siteID string) ([]CronJob, error) {
	rows, err := s.DB.Query(`SELECT id, site_id, schedule, command, enabled, created_at FROM cron_jobs WHERE site_id = ? ORDER BY created_at`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CronJob
	for rows.Next() {
		var j CronJob
		var en int
		if err := rows.Scan(&j.ID, &j.SiteID, &j.Schedule, &j.Command, &en, &j.CreatedAt); err != nil {
			return nil, err
		}
		j.Enabled = en == 1
		out = append(out, j)
	}
	if out == nil {
		out = []CronJob{}
	}
	return out, rows.Err()
}

func (s *Store) CreateCronJob(j CronJob) error {
	en := 0
	if j.Enabled {
		en = 1
	}
	_, err := s.DB.Exec(`INSERT INTO cron_jobs (id, site_id, schedule, command, enabled, created_at) VALUES (?,?,?,?,?,?)`,
		j.ID, j.SiteID, j.Schedule, j.Command, en, j.CreatedAt)
	return err
}

func (s *Store) DeleteCronJob(id, siteID string) error {
	res, err := s.DB.Exec(`DELETE FROM cron_jobs WHERE id = ? AND site_id = ?`, id, siteID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetCronJob(id string) (*CronJob, error) {
	row := s.DB.QueryRow(`SELECT id, site_id, schedule, command, enabled, created_at FROM cron_jobs WHERE id = ?`, id)
	var j CronJob
	var en int
	if err := row.Scan(&j.ID, &j.SiteID, &j.Schedule, &j.Command, &en, &j.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	j.Enabled = en == 1
	return &j, nil
}

