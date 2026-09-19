package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hebernet/hebernet/internal/store"
)

func (s *Server) handleStripeCheckout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain  string `json:"domain"`
		Email   string `json:"email"`
		AppType string `json:"app_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "json invalide")
		return
	}
	domain := strings.ToLower(strings.TrimSpace(body.Domain))
	email := strings.ToLower(strings.TrimSpace(body.Email))
	appType := strings.ToLower(strings.TrimSpace(body.AppType))
	if appType == "" {
		appType = "static"
	}
	if !domainRe.MatchString(domain) {
		writeErr(w, http.StatusBadRequest, "domaine invalide")
		return
	}
	if email == "" || !strings.Contains(email, "@") {
		writeErr(w, http.StatusBadRequest, "email invalide")
		return
	}
	switch appType {
	case "static", "php", "bludit", "hugo", "codeigniter":
	default:
		writeErr(w, http.StatusBadRequest, "app_type invalide")
		return
	}

	// Demo local sans clé Stripe : provision immédiate + page de suivi.
	if s.StripeSecretKey == "" {
		if !s.StripeCheckoutDemo {
			writeErr(w, http.StatusServiceUnavailable, "STRIPE_SECRET_KEY manquant (ou HEBERNET_CHECKOUT_DEMO=1)")
			return
		}
		sid := "cs_demo_" + randomHex(8)
		_ = s.Store.SaveStripePending(sid, domain, email, appType)
		res, mailErr, err := s.fulfillStripeOrder(sid, domain, email, appType)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		base := strings.TrimRight(s.VitrineURL, "/")
		if base == "" {
			base = "http://127.0.0.1:8088"
		}
		out := map[string]any{
			"demo":       true,
			"session_id": sid,
			"url":        base + "/paiement.html?session_id=" + url.QueryEscape(sid),
			"site_id":    res.Site.ID,
		}
		if mailErr != nil {
			out["mail"] = mailErr.Error()
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	success := strings.TrimRight(s.VitrineURL, "/") + "/paiement.html?session_id={CHECKOUT_SESSION_ID}"
	cancel := strings.TrimRight(s.VitrineURL, "/") + "/paiement.html?annule=1"
	if s.VitrineURL == "" {
		success = "http://127.0.0.1:8088/paiement.html?session_id={CHECKOUT_SESSION_ID}"
		cancel = "http://127.0.0.1:8088/paiement.html?annule=1"
	}
	cents := s.StripePriceCents
	if cents <= 0 {
		cents = 100
	}

	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", success)
	form.Set("cancel_url", cancel)
	form.Set("customer_email", email)
	form.Set("metadata[domain]", domain)
	form.Set("metadata[app_type]", appType)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", "eur")
	form.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(cents))
	form.Set("line_items[0][price_data][product_data][name]", "Hébernet — "+domain)

	req, err := http.NewRequest(http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.SetBasicAuth(s.StripeSecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "stripe: "+err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("stripe HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(raw))))
		return
	}
	var session struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &session); err != nil || session.URL == "" {
		writeErr(w, http.StatusBadGateway, "réponse stripe illisible")
		return
	}
	_ = s.Store.SaveStripePending(session.ID, domain, email, appType)
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": session.ID,
		"url":        session.URL,
	})
}

func (s *Server) handleStripeStatus(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("id"))
	if sid == "" {
		sid = strings.TrimSpace(r.URL.Query().Get("session_id"))
	}
	if sid == "" {
		writeErr(w, http.StatusBadRequest, "session_id requis")
		return
	}

	// Si le webhook n’a pas encore tourné, ou site en erreur : sync / retry via Stripe.
	domain, email, siteID, _, err := s.Store.GetStripeSession(sid)
	needSync := true
	if err == nil && siteID != "" {
		if site, e := s.Store.GetSite(siteID); e == nil && site.Status == "active" {
			needSync = false
		}
	}
	if errors.Is(err, store.ErrNotFound) {
		needSync = true
		err = nil
	}
	if needSync {
		if syncErr := s.syncCheckoutSession(sid); syncErr != nil {
			log.Printf("stripe status sync %s: %v", sid, syncErr)
		}
	}

	domain, email, siteID, _, err = s.Store.GetStripeSession(sid)
	if err == nil {
		status := "active"
		siteStatus := ""
		if siteID != "" {
			if site, e := s.Store.GetSite(siteID); e == nil {
				siteStatus = site.Status
				switch site.Status {
				case "provisioning":
					status = "provisioning"
				case "error":
					status = "error"
				default:
					status = "active"
				}
				domain = site.Domain
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"session_id":  sid,
			"status":      status,
			"domain":      domain,
			"email":       email,
			"site_id":     siteID,
			"site_status": siteStatus,
			"panel_url":   s.Mail.PanelURL,
			"message": func() string {
				if status == "error" {
					return "La préparation a échoué (souvent l’agent arrêté). Réessayez dans un instant — on retentera automatiquement."
				}
				return ""
			}(),
		})
		return
	}
	if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	pending, err := s.Store.GetStripePending(sid)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"session_id": sid,
			"status":     "pending",
			"domain":     pending.Domain,
			"email":      pending.Email,
			"message":    "Paiement reçu ou en cours — activation dans un instant.",
		})
		return
	}
	if !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sid,
		"status":     "unknown",
		"message":    "Session inconnue. Si vous venez de payer, patientez quelques secondes.",
	})
}

// syncCheckoutSession retrieves the Checkout Session from Stripe; if paid, fulfills.
func (s *Server) syncCheckoutSession(sessionID string) error {
	if s.StripeSecretKey == "" {
		return fmt.Errorf("pas de STRIPE_SECRET_KEY")
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.stripe.com/v1/checkout/sessions/"+url.PathEscape(sessionID), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.StripeSecretKey, "")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("stripe HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var session stripeCheckoutSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return err
	}
	if session.ID == "" {
		session.ID = sessionID
	}
	if session.PaymentStatus != "paid" && session.PaymentStatus != "no_payment_required" {
		return fmt.Errorf("paiement non soldé (%s)", session.PaymentStatus)
	}
	domain, err := session.domain()
	if err != nil {
		if p, e := s.Store.GetStripePending(sessionID); e == nil {
			domain = p.Domain
		} else {
			return err
		}
	}
	email, err := session.email()
	if err != nil {
		if p, e := s.Store.GetStripePending(sessionID); e == nil {
			email = p.Email
		} else {
			return err
		}
	}
	appType := session.appType()
	if p, e := s.Store.GetStripePending(sessionID); e == nil && p.AppType != "" {
		appType = p.AppType
	}
	_, _, err = s.fulfillStripeOrder(sessionID, domain, email, appType)
	return err
}
