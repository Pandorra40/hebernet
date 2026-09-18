package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// ensureFilebrowser prepares a per-site root marker; the API starts the Quantum process.
func (s *Server) ensureFilebrowser(p map[string]any, out map[string]any) error {
	user := str(p, "linux_user")
	if user == "" {
		return fmt.Errorf("linux_user required")
	}
	home := s.siteRoot(user)
	public := filepath.Join(home, "public_html")
	if err := os.MkdirAll(public, 0o755); err != nil {
		return err
	}
	out["root"] = home
	out["public_html"] = public
	return nil
}
