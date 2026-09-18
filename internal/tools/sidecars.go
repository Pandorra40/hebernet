package tools

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Quantum release pinned for the demo FM sidecar.
const (
	fbQuantumVersion = "v2.0.6-beta"
	fbQuantumURL     = "https://github.com/gtsteffaniak/filebrowser/releases/download/v2.0.6-beta/linux-amd64-filebrowser"
)

// Manager starts demo sidecars: Adminer (PHP) and FileBrowser Quantum.
type Manager struct {
	Root       string // repo root
	DataDir    string
	AdminerPHP string // path to adminer.php
	AdminerCSS string // path to Hébernet adminer.css
	FBBin      string // path to filebrowser binary
	// ListenAddr: interface for sidecars (0.0.0.0 on VM so le navigateur distant joigne ; 127.0.0.1 en démo locale).
	ListenAddr string

	mu      sync.Mutex
	phpCmd  *exec.Cmd
	fbCmd   *exec.Cmd
	phpPort int
	phpRoot string
	fbPort  int
	fbRoot  string
}

func (m *Manager) listenHost() string {
	if m.ListenAddr != "" {
		return m.ListenAddr
	}
	return "127.0.0.1"
}

func (m *Manager) AdminerURL(siteRel string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", m.phpPort, siteRel)
}

func (m *Manager) FilebrowserURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", m.fbPort)
}

func (m *Manager) EnsureAdminerPHP() error {
	dest := filepath.Join(m.Root, "third_party", "adminer.php")
	cssDest := filepath.Join(m.Root, "third_party", "adminer.css")
	themeSrc := filepath.Join(m.Root, "assets", "adminer.css")

	// Always refresh theme from assets (project design)
	if tb, err := os.ReadFile(themeSrc); err == nil {
		_ = os.MkdirAll(filepath.Dir(cssDest), 0o755)
		_ = os.WriteFile(cssDest, tb, 0o644)
		m.AdminerCSS = cssDest
	}

	if st, err := os.Stat(dest); err == nil && st.Size() > 1000 {
		m.AdminerPHP = dest
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	urls := []string{
		"https://github.com/vrana/adminer/releases/download/v5.4.1/adminer-5.4.1.php",
		"https://www.adminer.org/latest-en.php",
		"https://www.adminer.org/latest.php",
	}
	var last error
	for _, u := range urls {
		if err := download(u, dest); err != nil {
			last = err
			continue
		}
		m.AdminerPHP = dest
		return nil
	}
	return last
}

func (m *Manager) EnsureFilebrowserBin() error {
	dest := filepath.Join(m.Root, "third_party", "filebrowser")
	marker := dest + ".version"
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)

	need := true
	if st, err := os.Stat(dest); err == nil && st.Size() > 1000 {
		if b, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(b)) == fbQuantumVersion {
			need = false
		} else if out, err := exec.Command(dest, "version").CombinedOutput(); err == nil && strings.Contains(string(out), fbQuantumVersion) {
			_ = os.WriteFile(marker, []byte(fbQuantumVersion+"\n"), 0o644)
			need = false
		}
	}
	if !need {
		m.FBBin = dest
		return nil
	}

	tmp := dest + ".tmp"
	if err := download(fbQuantumURL, tmp); err != nil {
		return fmt.Errorf("téléchargement FileBrowser Quantum %s: %w", fbQuantumVersion, err)
	}
	_ = os.Chmod(tmp, 0o755)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(dest)
		if err2 := os.Rename(tmp, dest); err2 != nil {
			return err2
		}
	}
	_ = os.WriteFile(marker, []byte(fbQuantumVersion+"\n"), 0o644)
	m.FBBin = dest
	return nil
}

func (m *Manager) StartPHP(docRoot string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.phpCmd != nil && m.phpCmd.Process != nil && m.phpRoot == docRoot && m.phpPort > 0 {
		if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", m.phpPort), 100*time.Millisecond); err == nil {
			_ = c.Close()
			return m.phpPort, nil
		}
		_ = m.phpCmd.Process.Kill()
		_, _ = m.phpCmd.Process.Wait()
		m.phpCmd = nil
	}
	if m.phpCmd != nil && m.phpCmd.Process != nil {
		_ = m.phpCmd.Process.Kill()
		_, _ = m.phpCmd.Process.Wait()
		m.phpCmd = nil
	}

	port, err := freePort()
	if err != nil {
		return 0, err
	}
	sessDir := filepath.Join(m.DataDir, "php-sessions")
	if err := os.MkdirAll(sessDir, 0o700); err != nil {
		return 0, err
	}
	// session.save_path sous DataDir : writable (user hebernet + ProtectSystem=strict)
	cmd := exec.Command("php",
		"-d", "session.save_path="+sessDir,
		"-d", "session.use_strict_mode=1",
		"-S", fmt.Sprintf("%s:%d", m.listenHost(), port),
		"-t", docRoot,
	)
	cmd.Dir = docRoot
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	m.phpCmd = cmd
	m.phpPort = port
	m.phpRoot = docRoot
	time.Sleep(200 * time.Millisecond)
	return port, nil
}

func (m *Manager) StartFilebrowser(root string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fbCmd != nil && m.fbCmd.Process != nil && m.fbRoot == root {
		if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", m.fbPort), 100*time.Millisecond); err == nil {
			_ = c.Close()
			return m.fbPort, nil
		}
		_ = m.fbCmd.Process.Kill()
		_, _ = m.fbCmd.Process.Wait()
		m.fbCmd = nil
	}
	if m.fbCmd != nil && m.fbCmd.Process != nil {
		_ = m.fbCmd.Process.Kill()
		_, _ = m.fbCmd.Process.Wait()
		m.fbCmd = nil
	}
	if m.FBBin == "" {
		return 0, fmt.Errorf("filebrowser non installé")
	}
	port, err := freePort()
	if err != nil {
		return 0, err
	}

	cfgDir := filepath.Join(m.DataDir, "filebrowser-runtime")
	cacheDir := filepath.Join(m.DataDir, "filebrowser-cache")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.MkdirAll(cacheDir, 0o755)
	// Drop classic FileBrowser DB that blocks Quantum v2 startup.
	_ = os.Remove(filepath.Join(cfgDir, "database.db"))
	_ = os.Remove(filepath.Join(cfgDir, "filebrowser.db"))

	cfgPath := filepath.Join(cfgDir, "config.yaml")
	logPath := filepath.Join(m.DataDir, "filebrowser.log")
	// Quantum v2 config: http.port / http.listen (pas server.port)
	cfg := fmt.Sprintf(`http:
  port: %d
  listen: %q
server:
  cacheDir: %q
  database:
    path: "filebrowser.sqlite"
  sources:
    - path: %q
      name: "Site"
      config:
        defaultEnabled: true
auth:
  methods:
    noauth: true
frontend:
  name: "Hébernet Fichiers"
  disableUsedPercentage: true
`, port, m.listenHost(), cacheDir, root)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return 0, err
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(m.FBBin, "-c", cfgPath)
	cmd.Dir = cfgDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("start filebrowser: %w", err)
	}

	// Quantum indexing + PWA icons can take >10s on first boot.
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			_ = logFile.Close()
			b, _ := os.ReadFile(logPath)
			return 0, fmt.Errorf("filebrowser a quitté: %s", strings.TrimSpace(string(b)))
		}
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			_, _ = cmd.Process.Wait()
			_ = logFile.Close()
			b, _ := os.ReadFile(logPath)
			msg := strings.TrimSpace(string(b))
			if msg == "" {
				msg = "processus terminé sans log"
			}
			return 0, fmt.Errorf("filebrowser a quitté: %s", msg)
		}
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			m.fbCmd = cmd
			m.fbPort = port
			m.fbRoot = root
			return port, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	_ = logFile.Close()
	b, _ := os.ReadFile(logPath)
	return 0, fmt.Errorf("timeout démarrage filebrowser: %s", strings.TrimSpace(string(b)))
}

// SyncQuantumQuota prepares Quantum Storage Quotas (limitBytes) when the
// running build exposes the API (planned Quantum ≥ v2.1). Until then it is a
// no-op: Linux setquota remains the enforcement source of truth.
func (m *Manager) SyncQuantumQuota(limitBytes int64) (map[string]any, error) {
	m.mu.Lock()
	port := m.fbPort
	m.mu.Unlock()
	out := map[string]any{
		"limit_bytes": limitBytes,
		"applied":     false,
		"reason":      "FileBrowser Quantum Storage Quotas (limitBytes) non disponibles avant v2.1 — quota Hébernet via setquota",
	}
	if port == 0 || limitBytes <= 0 {
		return out, nil
	}
	// Probe future endpoints; ignore failures on v2.0.x.
	candidates := []string{
		fmt.Sprintf("http://127.0.0.1:%d/api/quotas", port),
		fmt.Sprintf("http://127.0.0.1:%d/api/settings/quotas", port),
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for _, u := range candidates {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = res.Body.Close()
		if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent {
			out["endpoint"] = u
			out["reason"] = "endpoint quota détecté — PATCH limitBytes à brancher quand le schéma API sera stable"
			return out, nil
		}
	}
	return out, nil
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "hebernet-demo/1.0")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d for %s", res.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, res.Body)
	return err
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
