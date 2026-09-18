package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) syncCron(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	if user == "" {
		return fmt.Errorf("linux_user required")
	}
	raw, _ := json.Marshal(p["jobs"])
	var jobs []struct {
		Schedule string `json:"schedule"`
		Command  string `json:"command"`
		Enabled  bool   `json:"enabled"`
	}
	_ = json.Unmarshal(raw, &jobs)

	var lines []string
	lines = append(lines, fmt.Sprintf("# Hébernet cron — %s", user))
	for _, j := range jobs {
		if !j.Enabled {
			continue
		}
		sched := strings.TrimSpace(j.Schedule)
		cmd := strings.TrimSpace(j.Command)
		if sched == "" || cmd == "" {
			continue
		}
		// basic safety: no newlines
		if strings.ContainsAny(sched, "\n\r") || strings.ContainsAny(cmd, "\n\r") {
			return fmt.Errorf("schedule/command invalides")
		}
		lines = append(lines, sched+" "+cmd)
	}
	body := strings.Join(lines, "\n") + "\n"

	if s.DryRun {
		path := filepath.Join(s.DataDir, "cron", user+".cron")
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		out["path"] = path
		out["lines"] = len(lines) - 1
		return nil
	}

	tmp := filepath.Join(os.TempDir(), "hebernet-"+user+".cron")
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := run("crontab", "-u", user, tmp); err != nil {
		return err
	}
	out["lines"] = len(lines) - 1
	return nil
}

func (s *Server) installAdminer(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	src := str(p, "adminer_src")
	cssSrc := str(p, "adminer_css")
	if user == "" || src == "" {
		return fmt.Errorf("linux_user and adminer_src required")
	}
	home := s.siteRoot(user)
	destDir := filepath.Join(home, "public_html", ".hebernet")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, "adminer.php")
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("lire adminer: %w", err)
	}
	if err := os.WriteFile(dest, b, 0o644); err != nil {
		return err
	}
	// Thème Hébernet : Adminer charge adminer.css / adminer-dark.css à côté du .php
	if cssSrc != "" {
		if cb, err := os.ReadFile(cssSrc); err == nil {
			_ = os.WriteFile(filepath.Join(destDir, "adminer.css"), cb, 0o644)
			_ = os.WriteFile(filepath.Join(destDir, "adminer-dark.css"), cb, 0o644)
		}
	}
	_ = os.WriteFile(filepath.Join(destDir, "index.html"), []byte(""), 0o644)
	out["path"] = dest
	out["url_path"] = "/.hebernet/adminer.php"
	out["theme"] = "hebernet"
	return nil
}
