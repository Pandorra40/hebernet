package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config for Resend transactional mail (optional).
type Config struct {
	APIKey     string
	From       string // e.g. Hébernet <ne-pas-repondre@example.fr>
	ReplyTo    string
	PanelURL   string
	ServerIP   string
}

type WelcomeParams struct {
	To           string
	Domain       string
	PanelEmail   string
	PanelPassword string
	SFTPUser     string
	SFTPPassword string
	DBName       string
	DBUser       string
	DBPassword   string
}

func (c Config) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != "" && strings.TrimSpace(c.From) != ""
}

func (c Config) SendWelcome(p WelcomeParams) error {
	if !c.Enabled() {
		return fmt.Errorf("mail non configuré (RESEND_API_KEY / HEBERNET_MAIL_FROM)")
	}
	body := buildWelcomeText(c, p)
	payload := map[string]any{
		"from":    c.From,
		"to":      []string{p.To},
		"subject": "Votre hébergement Hébernet — " + p.Domain,
		"text":    body,
	}
	if c.ReplyTo != "" {
		payload["reply_to"] = c.ReplyTo
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 300 {
		return fmt.Errorf("resend HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func buildWelcomeText(c Config, p WelcomeParams) string {
	panel := c.PanelURL
	if panel == "" {
		panel = "http://127.0.0.1:5173"
	}
	ip := c.ServerIP
	if ip == "" {
		ip = "(IP du serveur)"
	}
	var b strings.Builder
	b.WriteString("Bonjour,\n\nVotre hébergement Hébernet est en service pour ")
	b.WriteString(p.Domain)
	b.WriteString(".\n\n")
	b.WriteString("ACCÈS AU PANNEAU\n")
	b.WriteString("  Adresse      : " + panel + "\n")
	b.WriteString("  E-mail       : " + p.PanelEmail + "\n")
	b.WriteString("  Mot de passe : " + p.PanelPassword + "\n\n")
	b.WriteString("SFTP\n")
	b.WriteString("  Utilisateur  : " + p.SFTPUser + "\n")
	b.WriteString("  Mot de passe : " + p.SFTPPassword + "\n\n")
	if p.DBName != "" {
		b.WriteString("BASE DE DONNÉES\n")
		b.WriteString("  Serveur      : localhost\n")
		b.WriteString("  Base         : " + p.DBName + "\n")
		b.WriteString("  Utilisateur  : " + p.DBUser + "\n")
		b.WriteString("  Mot de passe : " + p.DBPassword + "\n\n")
	}
	b.WriteString("DNS (chez votre registrar)\n")
	b.WriteString("  " + p.Domain + "     → " + ip + "\n")
	b.WriteString("  www." + p.Domain + " → " + ip + "\n\n")
	b.WriteString("Le panneau est accessible tout de suite ; le site public l’est après propagation DNS.\n")
	return b.String()
}
