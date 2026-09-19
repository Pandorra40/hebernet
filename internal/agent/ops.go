package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func (s *Server) siteRoot(linuxUser string) string {
	if s.DryRun {
		return filepath.Join(s.DataDir, "homes", linuxUser)
	}
	return filepath.Join("/home", linuxUser)
}

func (s *Server) createSiteUser(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	if user == "" {
		return fmt.Errorf("linux_user required")
	}
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	if err := os.MkdirAll(public, 0o755); err != nil {
		return err
	}
	pass := randomPass(16)
	if s.DryRun {
		meta := filepath.Join(s.DataDir, "users", user+".txt")
		_ = os.MkdirAll(filepath.Dir(meta), 0o755)
		_ = os.WriteFile(meta, []byte("password="+pass+"\n"), 0o600)
		_ = os.WriteFile(filepath.Join(public, "index.html"), []byte("<h1>Site prêt — Hébernet</h1>\n"), 0o644)
	} else {
		if err := run("useradd", "-m", "-d", home, "-s", "/usr/sbin/nologin", user); err != nil {
			return err
		}
		if err := setPassword(user, pass); err != nil {
			return err
		}
		_ = os.MkdirAll(public, 0o755)
		// Stub statique : évite un 403 nginx (directory index forbidden) si aucun app one-click.
		_ = os.WriteFile(filepath.Join(public, "index.html"), []byte("<!DOCTYPE html><html lang=\"fr\"><head><meta charset=\"utf-8\"><title>Hébernet</title></head><body><h1>Site prêt — Hébernet</h1><p>Déposez vos fichiers dans <code>public_html/</code> via SFTP.</p></body></html>\n"), 0o644)
		_ = run("chown", "-R", user+":"+user, home)
	}
	out["home_path"] = home
	out["sftp_user"] = user
	out["sftp_password"] = pass
	return nil
}

func (s *Server) deleteSiteUser(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	home := s.siteRoot(user)
	if s.DryRun {
		_ = os.RemoveAll(home)
		_ = os.Remove(filepath.Join(s.DataDir, "users", user+".txt"))
		_ = os.Remove(filepath.Join(s.DataDir, "nginx", user+".conf"))
		out["deleted"] = true
		return nil
	}
	_ = run("userdel", "-r", user)
	out["deleted"] = true
	return nil
}

// destroySite removes home, nginx, php-fpm, ssl markers, and MariaDB database/user.
func (s *Server) destroySite(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	domain := str(p, "domain")
	dbName := str(p, "db_name")
	dbUser := str(p, "db_user")

	removed := []string{}

	if dbName != "" {
		if err := s.dropDatabase(dbName, dbUser); err != nil {
			return fmt.Errorf("drop database: %w", err)
		}
		removed = append(removed, "database")
	}

	if domain != "" {
		if s.DryRun {
			_ = os.Remove(filepath.Join(s.DataDir, "nginx", domain+".conf"))
			_ = os.Remove(filepath.Join(s.DataDir, "ssl", domain+".pem"))
			_ = os.Remove(filepath.Join(s.DataDir, "suspended", domain))
			_ = os.RemoveAll(filepath.Join(s.DataDir, "logs", domain))
		} else {
			avail := filepath.Join("/etc/nginx/sites-available", domain+".conf")
			enabled := filepath.Join("/etc/nginx/sites-enabled", domain+".conf")
			_ = os.Remove(enabled)
			_ = os.Remove(avail)
			_ = run("systemctl", "reload", "nginx")
		}
		removed = append(removed, "nginx", "ssl")
	}

	if user != "" {
		if s.DryRun {
			_ = os.Remove(filepath.Join(s.DataDir, "php-fpm", user+".conf"))
			_ = os.Remove(filepath.Join(s.DataDir, "quotas", user))
		} else {
			_ = os.Remove(filepath.Join("/etc/php/8.5/fpm/pool.d", "hebernet-"+user+".conf"))
			_ = run("systemctl", "reload", "php8.5-fpm")
		}
		if err := s.deleteSiteUser(map[string]any{"linux_user": user}, map[string]any{}); err != nil {
			return err
		}
		removed = append(removed, "home", "user", "php-fpm")
	}

	out["removed"] = removed
	out["ok"] = true
	return nil
}

func (s *Server) dropDatabase(name, user string) error {
	if name == "" {
		return nil
	}
	if s.DryRun {
		_ = os.Remove(filepath.Join(s.DataDir, "mysql", name+".sql"))
		if user != "" {
			_ = os.Remove(filepath.Join(s.DataDir, "mysql", user+".pass"))
		}
		return nil
	}
	sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", name)
	if user != "" {
		sql += fmt.Sprintf(" DROP USER IF EXISTS '%s'@'localhost';", escapeSQL(user))
	}
	sql += " FLUSH PRIVILEGES;"
	return run("mysql", "-e", sql)
}

func (s *Server) setQuota(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	mb := num(p, "quota_mb")
	if s.DryRun {
		path := filepath.Join(s.DataDir, "quotas", user)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(fmt.Sprintf("%d\n", mb)), 0o644)
		out["quota_mb"] = mb
		return nil
	}
	// soft=hard blocks in KB
	blocks := mb * 1024
	b := fmt.Sprintf("%d", blocks)
	// -a échoue souvent sur tmpfs ; on cible /home puis /
	var last error
	for _, fs := range []string{"-a", "/home", "/"} {
		args := []string{"-u", user, b, b, "0", "0"}
		if fs == "-a" {
			args = append(args, "-a")
		} else {
			args = append(args, fs)
		}
		if err := run("setquota", args...); err != nil {
			last = err
			continue
		}
		out["quota_mb"] = mb
		out["filesystem"] = fs
		return nil
	}
	return fmt.Errorf("setquota: %w (filesystem must support usrquota)", last)
}

func (s *Server) provisionNginx(p map[string]any, out map[string]any) error {
	domain := str(p, "domain")
	user := str(p, "linux_user")
	appType := str(p, "app_type")
	home := s.siteRoot(user)
	root := filepath.Join(home, "public_html")
	if appType == "laravel" || appType == "codeigniter" {
		root = filepath.Join(home, "public_html", "public")
	}
	php := appType != "static" && appType != "hugo"
	conf := renderNginx(domain, root, user, php)
	var dest string
	if s.DryRun {
		dest = filepath.Join(s.DataDir, "nginx", domain+".conf")
		_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	} else {
		dest = filepath.Join("/etc/nginx/sites-available", domain+".conf")
	}
	if err := os.WriteFile(dest, []byte(conf), 0o644); err != nil {
		return err
	}
	if !s.DryRun {
		link := filepath.Join("/etc/nginx/sites-enabled", domain+".conf")
		_ = os.Remove(link)
		if err := os.Symlink(dest, link); err != nil {
			return err
		}
		if err := run("nginx", "-t"); err != nil {
			return err
		}
		_ = run("systemctl", "reload", "nginx")
	}
	out["nginx_conf"] = dest
	out["docroot"] = root
	return nil
}

func (s *Server) provisionPHP(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	version := str(p, "php_version")
	if version == "" {
		version = "8.5"
	}
	pool := fmt.Sprintf(`[hebernet-%s]
user = %s
group = %s
listen = /run/php/php%s-fpm-%s.sock
listen.owner = www-data
listen.group = www-data
pm = ondemand
pm.max_children = 5
pm.process_idle_timeout = 10s
pm.max_requests = 500
php_admin_value[memory_limit] = 128M
php_admin_value[open_basedir] = %s:/tmp
`, user, user, user, version, user, s.siteRoot(user))
	var dest string
	if s.DryRun {
		dest = filepath.Join(s.DataDir, "php-fpm", user+".conf")
		_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	} else {
		dest = filepath.Join("/etc/php", version, "fpm/pool.d", "hebernet-"+user+".conf")
	}
	if err := os.WriteFile(dest, []byte(pool), 0o644); err != nil {
		return err
	}
	if !s.DryRun {
		_ = run("systemctl", "reload", "php"+version+"-fpm")
	}
	// Stub PHP : OpProvisionPHP ne déployait que le pool → public_html vide → 403 nginx.
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	_ = os.MkdirAll(public, 0o755)
	indexPHP := filepath.Join(public, "index.php")
	if _, err := os.Stat(indexPHP); err != nil {
		stub := `<?php
declare(strict_types=1);
header('Content-Type: text/html; charset=utf-8');
echo "<!DOCTYPE html><html lang=\"fr\"><head><meta charset=\"utf-8\"><title>Hébernet PHP</title></head><body>";
echo "<h1>Site PHP — Hébernet</h1>";
echo "<p>PHP " . PHP_VERSION . " · déposez vos fichiers dans <code>public_html/</code> via SFTP.</p>";
echo "</body></html>\n";
`
		if err := os.WriteFile(indexPHP, []byte(stub), 0o644); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(public, "index.html")) // préférer index.php
		if !s.DryRun {
			_ = run("chown", "-R", user+":"+user, home)
		}
	}
	out["pool"] = dest
	out["php_version"] = version
	out["docroot"] = public
	return nil
}

func (s *Server) provisionWordPress(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	_ = os.MkdirAll(public, 0o755)
	if s.DryRun {
		stub := `<?php
// Stub WordPress — mode démo Hébernet
echo "<h1>WordPress (démo)</h1><p>Remplacé par un vrai téléchargement wp-cli en prod.</p>";
`
		if err := os.WriteFile(filepath.Join(public, "index.php"), []byte(stub), 0o644); err != nil {
			return err
		}
		out["method"] = "stub"
		return nil
	}
	archive := filepath.Join(os.TempDir(), "wordpress-latest.tar.gz")
	if err := run("curl", "-fsSL", "https://wordpress.org/latest.tar.gz", "-o", archive); err != nil {
		return err
	}
	if err := run("tar", "-xzf", archive, "-C", home); err != nil {
		return err
	}
	_ = run("bash", "-c", fmt.Sprintf("rm -rf %q/* && mv %q/wordpress/* %q/ && rmdir %q/wordpress", public, home, public, home))
	_ = run("chown", "-R", user+":"+user, home)
	out["method"] = "wordpress.org"
	return nil
}

func (s *Server) provisionLaravel(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	dbName := str(p, "db_name")
	dbUser := str(p, "db_user")
	dbPass := str(p, "db_password")
	home := s.siteRoot(user)
	publicHTML := filepath.Join(home, "public_html")
	pub := filepath.Join(publicHTML, "public")
	_ = os.MkdirAll(pub, 0o755)
	stub := `<?php
echo "<h1>Laravel (Hébernet)</h1><p>Docroot = public/. Installez via <code>composer create-project laravel/laravel</code> si composer est disponible.</p>";
`
	if s.DryRun {
		_ = os.WriteFile(filepath.Join(publicHTML, "artisan"), []byte("#!/usr/bin/env php\n<?php // artisan stub\n"), 0o755)
		_ = os.WriteFile(filepath.Join(pub, "index.php"), []byte(stub), 0o644)
		out["method"] = "stub"
		out["docroot"] = pub
		return nil
	}
	if _, err := exec.LookPath("composer"); err == nil {
		_ = os.RemoveAll(publicHTML)
		if err := run("composer", "create-project", "--prefer-dist", "laravel/laravel", publicHTML); err == nil {
			if dbName != "" && dbUser != "" {
				if dbPass == "" {
					return fmt.Errorf("laravel: mot de passe BDD manquant")
				}
				if err := configureLaravelEnv(publicHTML, dbName, dbUser, dbPass); err != nil {
					return fmt.Errorf("laravel .env mysql: %w", err)
				}
				// Vérifie que MariaDB accepte ce couple avant migrate
				check := exec.Command("mysql", "-u", dbUser, "-p"+dbPass, "-h", "127.0.0.1", "-e", "SELECT 1", dbName)
				if outCheck, err := check.CombinedOutput(); err != nil {
					return fmt.Errorf("laravel mysql auth: %w (%s)", err, strings.TrimSpace(string(outCheck)))
				}
				cmd := exec.Command("bash", "-c", fmt.Sprintf(
					`cd %q && php artisan key:generate --force && php artisan migrate --force`,
					publicHTML,
				))
				migOut, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("laravel migrate: %w (%s)", err, strings.TrimSpace(string(migOut)))
				}
				out["migrated"] = true
				out["db"] = "mysql"
			} else {
				// Pas de BDD : sessions fichier pour éviter l’erreur « no such table: sessions »
				_ = patchLaravelEnvKey(publicHTML, "SESSION_DRIVER", "file")
				_ = run("bash", "-c", fmt.Sprintf(`cd %q && php artisan key:generate --force 2>/dev/null; true`, publicHTML))
				out["db"] = "none"
			}
			_ = run("chown", "-R", user+":"+user, home)
			out["method"] = "composer"
			out["docroot"] = filepath.Join(publicHTML, "public")
			return nil
		}
	}
	// Sans composer : page d’accueil (le site reste actif)
	_ = os.MkdirAll(pub, 0o755)
	_ = os.WriteFile(filepath.Join(pub, "index.php"), []byte(stub), 0o644)
	_ = run("chown", "-R", user+":"+user, home)
	out["method"] = "stub"
	out["docroot"] = pub
	return nil
}

func configureLaravelEnv(appRoot, dbName, dbUser, dbPass string) error {
	if err := patchLaravelEnvKey(appRoot, "DB_CONNECTION", "mysql"); err != nil {
		return err
	}
	_ = patchLaravelEnvKey(appRoot, "DB_HOST", "127.0.0.1")
	_ = patchLaravelEnvKey(appRoot, "DB_PORT", "3306")
	_ = patchLaravelEnvKey(appRoot, "DB_DATABASE", dbName)
	_ = patchLaravelEnvKey(appRoot, "DB_USERNAME", dbUser)
	_ = patchLaravelEnvKey(appRoot, "DB_PASSWORD", quoteEnvValue(dbPass))
	_ = patchLaravelEnvKey(appRoot, "SESSION_DRIVER", "database")
	// Évite que Laravel garde un DB_URL sqlite
	_ = patchLaravelEnvKey(appRoot, "DB_URL", "")
	return nil
}

func quoteEnvValue(v string) string {
	if v == "" {
		return `""`
	}
	if strings.ContainsAny(v, " #\"'\\") {
		return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return v
}

func patchLaravelEnvKey(appRoot, key, val string) error {
	envPath := filepath.Join(appRoot, ".env")
	b, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	prefix := key + "="
	found := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, prefix) || strings.HasPrefix(trim, "#"+prefix) {
			lines[i] = prefix + val
			found = true
		}
	}
	if !found {
		lines = append(lines, prefix+val)
	}
	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o640)
}

func (s *Server) provisionPrestaShop(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	_ = os.MkdirAll(public, 0o755)
	if s.DryRun {
		stub := `<?php
echo "<h1>PrestaShop (démo Hébernet)</h1><p>Archive officielle + install wizard en prod.</p>";
`
		if err := os.WriteFile(filepath.Join(public, "index.php"), []byte(stub), 0o644); err != nil {
			return err
		}
		out["method"] = "stub"
		return nil
	}

	// Package installable (pas le zipball source GitHub « latest » qui n’a pas d’asset).
	const psURL = "https://github.com/PrestaShop/PrestaShop/releases/download/8.2.1/prestashop_8.2.1.zip"
	archive := filepath.Join(os.TempDir(), "prestashop_8.2.1.zip")
	extract := filepath.Join(os.TempDir(), "prestashop-extract")
	_ = os.RemoveAll(extract)
	_ = os.MkdirAll(extract, 0o755)

	if err := run("curl", "-fsSL", "-L", psURL, "-o", archive); err != nil {
		return fmt.Errorf("prestashop download: %w (%s)", err, psURL)
	}
	if err := run("unzip", "-q", "-o", archive, "-d", extract); err != nil {
		return fmt.Errorf("prestashop unzip: %w", err)
	}
	// L’archive officielle contient souvent un prestashop.zip imbriqué.
	nested := filepath.Join(extract, "prestashop.zip")
	if _, err := os.Stat(nested); err == nil {
		if err := run("unzip", "-q", "-o", nested, "-d", public); err != nil {
			return fmt.Errorf("prestashop nested unzip: %w", err)
		}
	} else {
		// Sinon copier le contenu extrait vers public_html
		_ = run("bash", "-c", fmt.Sprintf("rm -rf %q/* && cp -a %q/. %q/", public, extract, public))
	}
	_ = run("chown", "-R", user+":"+user, home)
	out["method"] = "zip"
	out["version"] = "8.2.1"
	out["source"] = psURL
	return nil
}

func (s *Server) provisionBludit(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	_ = os.MkdirAll(public, 0o755)
	if s.DryRun {
		_ = os.WriteFile(filepath.Join(public, "index.php"), []byte("<?php echo '<h1>Bludit (démo)</h1>';\n"), 0o644)
		out["method"] = "stub"
		return nil
	}
	ver := latestGitHubTag("bludit/bludit", "3.22.0")
	url := "https://github.com/bludit/bludit/archive/refs/tags/" + ver + ".zip"
	archive := filepath.Join(os.TempDir(), "bludit-"+ver+".zip")
	extract := filepath.Join(os.TempDir(), "bludit-extract-"+ver)
	_ = os.RemoveAll(extract)
	if err := run("curl", "-fsSL", "-L", url, "-o", archive); err != nil {
		return fmt.Errorf("bludit download: %w", err)
	}
	if err := run("unzip", "-q", "-o", archive, "-d", extract); err != nil {
		return fmt.Errorf("bludit unzip: %w", err)
	}
	src := filepath.Join(extract, "bludit-"+ver)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("bludit extract missing: %w", err)
	}
	_ = run("bash", "-c", fmt.Sprintf("rm -rf %q/* && cp -a %q/. %q/", public, src, public))
	_ = run("chown", "-R", user+":"+user, home)
	out["method"] = "github"
	out["version"] = ver
	return nil
}

func (s *Server) provisionHugo(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	src := filepath.Join(home, "hugo-site")
	_ = os.MkdirAll(public, 0o755)
	if s.DryRun {
		_ = os.WriteFile(filepath.Join(public, "index.html"), []byte("<h1>Hugo (démo Hébernet)</h1>\n"), 0o644)
		out["method"] = "stub"
		return nil
	}
	hugoBin, ver, err := s.ensureHugoBinary()
	if err != nil {
		return err
	}
	_ = os.RemoveAll(src)
	if err := run(hugoBin, "new", "site", src); err != nil {
		return fmt.Errorf("hugo new site: %w", err)
	}
	_ = os.MkdirAll(filepath.Join(src, "content"), 0o755)
	_ = os.MkdirAll(filepath.Join(src, "layouts"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "content", "_index.md"), []byte("---\ntitle: \"Site Hébernet\"\n---\n\nBienvenue. Éditez `hugo-site/` puis relancez `hugo`.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "layouts", "index.html"), []byte("<!DOCTYPE html>\n<html lang=\"fr\"><head><meta charset=\"utf-8\"><title>{{ .Title }}</title></head>\n<body><h1>{{ .Title }}</h1>{{ .Content }}</body></html>\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "hugo.toml"), []byte("baseURL = '/'\nlanguageCode = 'fr-fr'\ntitle = 'Site Hébernet'\n"), 0o644)
	if err := run(hugoBin, "--source", src, "--destination", public, "--cleanDestinationDir"); err != nil {
		return fmt.Errorf("hugo build: %w", err)
	}
	_ = run("chown", "-R", user+":"+user, home)
	out["method"] = "hugo"
	out["version"] = ver
	out["source_dir"] = src
	out["docroot"] = public
	return nil
}

func (s *Server) ensureHugoBinary() (bin string, ver string, err error) {
	ver = latestGitHubTag("gohugoio/hugo", "0.166.0")
	ver = strings.TrimPrefix(ver, "v")
	dir := filepath.Join(s.DataDir, "bin")
	_ = os.MkdirAll(dir, 0o755)
	dest := filepath.Join(dir, "hugo")
	marker := dest + ".version"
	if b, e := os.ReadFile(marker); e == nil && strings.TrimSpace(string(b)) == ver {
		if _, e2 := os.Stat(dest); e2 == nil {
			return dest, ver, nil
		}
	}
	url := fmt.Sprintf("https://github.com/gohugoio/hugo/releases/download/v%s/hugo_%s_linux-amd64.tar.gz", ver, ver)
	archive := filepath.Join(os.TempDir(), "hugo-"+ver+".tgz")
	if err := run("curl", "-fsSL", "-L", url, "-o", archive); err != nil {
		return "", "", fmt.Errorf("hugo download: %w", err)
	}
	extract := filepath.Join(os.TempDir(), "hugo-extract-"+ver)
	_ = os.RemoveAll(extract)
	_ = os.MkdirAll(extract, 0o755)
	if err := run("tar", "-xzf", archive, "-C", extract); err != nil {
		return "", "", fmt.Errorf("hugo extract: %w", err)
	}
	if err := run("install", "-m", "755", filepath.Join(extract, "hugo"), dest); err != nil {
		return "", "", err
	}
	_ = os.WriteFile(marker, []byte(ver+"\n"), 0o644)
	return dest, ver, nil
}

func (s *Server) provisionCodeIgniter(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	dbName := str(p, "db_name")
	dbUser := str(p, "db_user")
	dbPass := str(p, "db_password")
	home := s.siteRoot(user)
	publicHTML := filepath.Join(home, "public_html")
	pub := filepath.Join(publicHTML, "public")
	_ = os.MkdirAll(pub, 0o755)
	if s.DryRun {
		_ = os.WriteFile(filepath.Join(pub, "index.php"), []byte("<?php echo '<h1>CodeIgniter (démo)</h1>';\n"), 0o644)
		out["method"] = "stub"
		out["docroot"] = pub
		return nil
	}
	if _, err := exec.LookPath("composer"); err != nil {
		return fmt.Errorf("codeigniter: composer requis")
	}
	_ = os.RemoveAll(publicHTML)
	// Dernière appstarter stable Packagist (sans pin = toujours la plus récente)
	if err := run("composer", "create-project", "--prefer-dist", "codeigniter4/appstarter", publicHTML); err != nil {
		return fmt.Errorf("codeigniter composer: %w", err)
	}
	if dbName != "" && dbUser != "" {
		if dbPass == "" {
			return fmt.Errorf("codeigniter: mot de passe BDD manquant")
		}
		_ = patchCIEnv(publicHTML, dbName, dbUser, dbPass)
	}
	_ = run("chown", "-R", user+":"+user, home)
	out["method"] = "composer"
	out["docroot"] = pub
	return nil
}

// latestGitHubTag lit /releases/latest ; fallback si API KO / rate-limit.
func latestGitHubTag(repo, fallback string) string {
	cmd := exec.Command("curl", "-fsSL", "-H", "Accept: application/vnd.github+json",
		"https://api.github.com/repos/"+repo+"/releases/latest")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fallback
	}
	var meta struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(b, &meta); err != nil || meta.TagName == "" {
		return fallback
	}
	return strings.TrimSpace(meta.TagName)
}

func patchCIEnv(appRoot, dbName, dbUser, dbPass string) error {
	envPath := filepath.Join(appRoot, "env")
	dst := filepath.Join(appRoot, ".env")
	if _, err := os.Stat(dst); err != nil {
		_ = run("cp", envPath, dst)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		return err
	}
	text := string(b)
	repls := map[string]string{
		"CI_ENVIRONMENT = production": "CI_ENVIRONMENT = production",
		"# database.default.hostname":  "database.default.hostname",
		"# database.default.database":  "database.default.database",
		"# database.default.username":  "database.default.username",
		"# database.default.password":  "database.default.password",
		"# database.default.DBDriver":  "database.default.DBDriver",
	}
	for old, neu := range repls {
		text = strings.ReplaceAll(text, old, neu)
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "database.default.hostname"):
			lines[i] = "database.default.hostname = 127.0.0.1"
		case strings.HasPrefix(trim, "database.default.database"):
			lines[i] = "database.default.database = " + dbName
		case strings.HasPrefix(trim, "database.default.username"):
			lines[i] = "database.default.username = " + dbUser
		case strings.HasPrefix(trim, "database.default.password"):
			lines[i] = "database.default.password = " + dbPass
		case strings.HasPrefix(trim, "database.default.DBDriver"):
			lines[i] = "database.default.DBDriver = MySQLi"
		}
	}
	return os.WriteFile(dst, []byte(strings.Join(lines, "\n")), 0o640)
}

func patchCIEnvPassword(appRoot, pass string) error {
	envPath := filepath.Join(appRoot, ".env")
	b, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "database.default.password") {
			lines[i] = "database.default.password = " + pass
		}
	}
	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o640)
}

func (s *Server) issueSSL(p map[string]any, out map[string]any) error {
	domain := str(p, "domain")
	if s.DryRun {
		marker := filepath.Join(s.DataDir, "ssl", domain+".pem")
		_ = os.MkdirAll(filepath.Dir(marker), 0o755)
		_ = os.WriteFile(marker, []byte("DEMO CERT\n"), 0o644)
		out["ssl"] = true
		out["demo"] = true
		return nil
	}
	if err := run("certbot", "--nginx", "-d", domain, "--non-interactive", "--agree-tos", "-m", "admin@"+domain); err != nil {
		return err
	}
	out["ssl"] = true
	return nil
}

func (s *Server) suspendSite(p map[string]any, suspend bool, out map[string]any) error {
	domain := str(p, "domain")
	flag := filepath.Join(s.DataDir, "suspended", domain)
	_ = os.MkdirAll(filepath.Dir(flag), 0o755)
	if suspend {
		_ = os.WriteFile(flag, []byte("1\n"), 0o644)
	} else {
		_ = os.Remove(flag)
	}
	out["suspended"] = suspend
	return nil
}

func (s *Server) resetSFTP(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	pass := randomPass(16)
	if s.DryRun {
		meta := filepath.Join(s.DataDir, "users", user+".txt")
		_ = os.MkdirAll(filepath.Dir(meta), 0o755)
		_ = os.WriteFile(meta, []byte("password="+pass+"\n"), 0o600)
	} else {
		if err := setPassword(user, pass); err != nil {
			return err
		}
	}
	out["sftp_password"] = pass
	return nil
}

func (s *Server) createDatabase(p map[string]any, out map[string]any) error {
	name := str(p, "db_name")
	user := str(p, "db_user")
	pass := randomPass(20)
	if name == "" || user == "" {
		return fmt.Errorf("db_name and db_user required")
	}
	if s.DryRun {
		path := filepath.Join(s.DataDir, "mysql", name+".sql")
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(fmt.Sprintf("-- demo db %s user %s\n", name, user)), 0o600)
		out["db_name"] = name
		out["db_user"] = user
		out["db_password"] = pass
		return nil
	}
	sql := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s`; "+
			"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; "+
			"ALTER USER '%s'@'localhost' IDENTIFIED BY '%s'; "+
			"GRANT ALL ON `%s`.* TO '%s'@'localhost'; FLUSH PRIVILEGES;",
		name, user, escapeSQL(pass), user, escapeSQL(pass), name, user,
	)
	if err := run("mysql", "-e", sql); err != nil {
		return err
	}
	out["db_name"] = name
	out["db_user"] = user
	out["db_password"] = pass
	return nil
}

func (s *Server) resetDBPass(p map[string]any, out map[string]any) error {
	user := str(p, "db_user")
	if user == "" {
		return fmt.Errorf("db_user required")
	}
	pass := randomPass(20)
	if s.DryRun {
		path := filepath.Join(s.DataDir, "mysql", user+".pass")
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(pass+"\n"), 0o600)
		out["db_password"] = pass
		return nil
	}
	sql := fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;", escapeSQL(user), escapeSQL(pass))
	if err := run("mysql", "-e", sql); err != nil {
		return err
	}
	// Aligne Laravel .env / wp-config si on connaît le site (évite Adminer ≠ app)
	appType := str(p, "app_type")
	linuxUser := str(p, "linux_user")
	home := str(p, "home_path")
	if home == "" && linuxUser != "" {
		home = s.siteRoot(linuxUser)
	}
	if home != "" {
		appRoot := filepath.Join(home, "public_html")
		switch appType {
		case "laravel":
			_ = patchLaravelEnvKey(appRoot, "DB_PASSWORD", quoteEnvValue(pass))
		case "codeigniter":
			_ = patchCIEnvPassword(appRoot, pass)
		case "wordpress":
			_ = patchWPConfigDBPass(appRoot, pass)
		}
	}
	out["db_password"] = pass
	return nil
}

func patchWPConfigDBPass(appRoot, pass string) error {
	cfg := filepath.Join(appRoot, "wp-config.php")
	b, err := os.ReadFile(cfg)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`define\(\s*['"]DB_PASSWORD['"]\s*,\s*['"][^'"]*['"]\s*\)`)
	repl := fmt.Sprintf("define('DB_PASSWORD', '%s')", strings.ReplaceAll(pass, "'", "\\'"))
	if !re.Match(b) {
		return fmt.Errorf("DB_PASSWORD not found in wp-config.php")
	}
	return os.WriteFile(cfg, re.ReplaceAll(b, []byte(repl)), 0o640)
}

func (s *Server) quotaUsage(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	home := s.siteRoot(user)
	var size int64
	_ = filepath.Walk(home, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	usedMB := int(size / (1024 * 1024))
	if usedMB == 0 && size > 0 {
		usedMB = 1
	}
	out["used_mb"] = usedMB
	return nil
}

func renderNginx(domain, root, user string, php bool) string {
	phpBlock := ""
	if php {
		phpBlock = fmt.Sprintf(`
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.5-fpm-%s.sock;
    }
`, user)
	}
	return fmt.Sprintf(`# Hébernet — %s
server {
    listen 80;
    server_name %s;
    root %s;
    index index.php index.html;
    client_max_body_size 64m;
    location / {
        try_files $uri $uri/ /index.php?$args;
    }
%s}
`, domain, domain, root, phpBlock)
}

func randomPass(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func setPassword(user, pass string) error {
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(user + ":" + pass + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chpasswd: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
