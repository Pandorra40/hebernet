package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('admin','client')),
  display_name TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS packages (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  disk_mb INTEGER NOT NULL,
  max_sites INTEGER NOT NULL DEFAULT 1,
  max_databases INTEGER NOT NULL DEFAULT 1,
  description TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sites (
  id TEXT PRIMARY KEY,
  domain TEXT NOT NULL UNIQUE,
  app_type TEXT NOT NULL CHECK(app_type IN ('static','php','bludit','hugo','codeigniter','wordpress','laravel','prestashop')),
  owner_id TEXT NOT NULL REFERENCES users(id),
  package_id TEXT NOT NULL REFERENCES packages(id),
  linux_user TEXT NOT NULL,
  home_path TEXT NOT NULL,
  php_version TEXT NOT NULL DEFAULT '8.5',
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','suspended','provisioning','error')),
  ssl_enabled INTEGER NOT NULL DEFAULT 0,
  db_name TEXT NOT NULL DEFAULT '',
  db_user TEXT NOT NULL DEFAULT '',
  sftp_user TEXT NOT NULL DEFAULT '',
  quota_mb INTEGER NOT NULL DEFAULT 0,
  quota_used_mb INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cron_jobs (
  id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  schedule TEXT NOT NULL,
  command TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stripe_sessions (
  session_id TEXT PRIMARY KEY,
  domain TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  site_id TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stripe_pending (
  session_id TEXT PRIMARY KEY,
  domain TEXT NOT NULL,
  email TEXT NOT NULL,
  app_type TEXT NOT NULL DEFAULT 'static',
  created_at TEXT NOT NULL
);
`
	if _, err := s.DB.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := s.expandAppTypes(); err != nil {
		return err
	}
	if err := s.ensurePackageColumns(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensurePackageColumns() error {
	_, err := s.DB.Exec(`ALTER TABLE packages ADD COLUMN max_databases INTEGER NOT NULL DEFAULT 1`)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		// SQLite says "duplicate column name" when already present
		if !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("migrate max_databases: %w", err)
		}
	}
	return nil
}

// expandAppTypes rebuilds sites table when CHECK rejects les nouveaux app_type.
func (s *Store) expandAppTypes() error {
	_, err := s.DB.Exec(`INSERT INTO sites (
		id, domain, app_type, owner_id, package_id, linux_user, home_path, php_version, status,
		ssl_enabled, db_name, db_user, sftp_user, quota_mb, quota_used_mb, created_at, updated_at
	) SELECT
		'__hebernet_apptype_probe__', '__hebernet_probe__.invalid', 'bludit',
		(SELECT id FROM users LIMIT 1), (SELECT id FROM packages LIMIT 1),
		'_probe', '/tmp', '8.5', 'active', 0, '', '', '', 0, 0, ?, ?
	WHERE EXISTS (SELECT 1 FROM users) AND EXISTS (SELECT 1 FROM packages)`, Now(), Now())
	if err == nil {
		_, _ = s.DB.Exec(`DELETE FROM sites WHERE id = '__hebernet_apptype_probe__'`)
		return nil
	}
	// Rebuild table with new CHECK
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	check := AllAppTypesSQL()
	stmts := []string{
		`CREATE TABLE sites_new (
  id TEXT PRIMARY KEY,
  domain TEXT NOT NULL UNIQUE,
  app_type TEXT NOT NULL CHECK(app_type IN (` + check + `)),
  owner_id TEXT NOT NULL REFERENCES users(id),
  package_id TEXT NOT NULL REFERENCES packages(id),
  linux_user TEXT NOT NULL,
  home_path TEXT NOT NULL,
  php_version TEXT NOT NULL DEFAULT '8.5',
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','suspended','provisioning','error')),
  ssl_enabled INTEGER NOT NULL DEFAULT 0,
  db_name TEXT NOT NULL DEFAULT '',
  db_user TEXT NOT NULL DEFAULT '',
  sftp_user TEXT NOT NULL DEFAULT '',
  quota_mb INTEGER NOT NULL DEFAULT 0,
  quota_used_mb INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)`,
		`INSERT INTO sites_new SELECT * FROM sites`,
		`DROP TABLE sites`,
		`ALTER TABLE sites_new RENAME TO sites`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("expand app_type: %w", err)
		}
	}
	return tx.Commit()
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
