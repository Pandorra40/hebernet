package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Server) readLogs(p map[string]any, out map[string]any) error {
	domain := str(p, "domain")
	kind := str(p, "kind")
	if kind == "" {
		kind = "access"
	}
	var path string
	if s.DryRun {
		dir := filepath.Join(s.DataDir, "logs", domain)
		_ = os.MkdirAll(dir, 0o755)
		path = filepath.Join(dir, kind+".log")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			sample := fmt.Sprintf("# Logs %s — %s (démo)\n127.0.0.1 - - [17/Sep/2026:18:00:01 +0000] \"GET / HTTP/1.1\" 200 312\n127.0.0.1 - - [17/Sep/2026:18:01:12 +0000] \"GET /favicon.ico HTTP/1.1\" 404 0\n", kind, domain)
			_ = os.WriteFile(path, []byte(sample), 0o644)
		}
	} else {
		path = filepath.Join("/var/log/nginx", domain+"-"+kind+".log")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		out["lines"] = "(aucun log pour l’instant)"
		out["path"] = path
		return nil
	}
	// last ~8 KB
	if len(b) > 8192 {
		b = b[len(b)-8192:]
	}
	out["lines"] = string(b)
	out["path"] = path
	return nil
}
