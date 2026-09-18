package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hebernet/hebernet/internal/mail"
	"github.com/hebernet/hebernet/internal/store"
)

const stripeSigTolerance = 300 * time.Second

func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
		return
	}
	if s.StripeWebhookSecret == "" {
		writeErr(w, http.StatusServiceUnavailable, "STRIPE_WEBHOOK_SECRET non configuré")
		return
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "corps illisible")
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	if err := verifyStripeSignature(payload, sig, s.StripeWebhookSecret, stripeSigTolerance); err != nil {
		log.Printf("stripe webhook signature: %v", err)
		writeErr(w, http.StatusBadRequest, "signature invalide")
		return
	}

	var evt struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		writeErr(w, http.StatusBadRequest, "json invalide")
		return
	}
	if evt.Type != "checkout.session.completed" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignore", "type": evt.Type})
		return
	}

	var session stripeCheckoutSession
	if err := json.Unmarshal(evt.Data.Object, &session); err != nil {
		writeErr(w, http.StatusBadRequest, "session illisible")
		return
	}
	if session.ID == "" {
		writeErr(w, http.StatusBadRequest, "id session absent")
		return
	}

	done, err := s.Store.StripeSessionProcessed(session.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if done {
		writeJSON(w, http.StatusOK, map[string]string{"status": "deja_traite"})
		return
	}
	if session.PaymentStatus != "" && session.PaymentStatus != "paid" && session.PaymentStatus != "no_payment_required" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "paiement_non_solde"})
		return
	}

	domain, err := session.domain()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := session.email()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	result, mailErr, err := s.fulfillStripeOrder(session.ID, domain, email, session.appType())
	if err != nil {
		log.Printf("stripe provision %s: %v", session.ID, err)
		writeErr(w, http.StatusInternalServerError, "provision echouee")
		return
	}
	status := "cree"
	if result.AlreadyExists {
		status = "existant"
	}
	out := map[string]any{
		"status": status,
		"domain": domain,
		"site_id": result.Site.ID,
	}
	if mailErr != nil {
		out["mail"] = mailErr.Error()
	} else if s.Mail.Enabled() {
		out["mail"] = "envoye"
	} else {
		out["mail"] = "desactive"
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) fulfillStripeOrder(sessionID, domain, email, appType string) (*provisionResult, error, error) {
	pkgName := s.StripePackageName
	if pkgName == "" {
		pkgName = "Starter"
	}
	pkg, err := s.Store.GetPackageByName(pkgName)
	if err != nil {
		return nil, nil, fmt.Errorf("package %q: %w", pkgName, err)
	}

	owner, panelPass, err := s.ensureClientUser(email)
	if err != nil {
		return nil, nil, err
	}

	res, err := s.provisionSite(provisionRequest{
		Domain: domain, AppType: appType, OwnerID: owner.ID, PackageID: pkg.ID, SkipLimits: true,
	})
	if err != nil {
		return nil, nil, err
	}
	_ = s.Store.MarkStripeSession(sessionID, domain, email, res.Site.ID)

	var mailErr error
	if s.Mail.Enabled() && !res.AlreadyExists {
		passForMail := panelPass
		if passForMail == "" {
			passForMail = "(mot de passe inchangé — utilisez « mot de passe oublié » ou contactez le support)"
		}
		mailErr = s.Mail.SendWelcome(mail.WelcomeParams{
			To:            email,
			Domain:        domain,
			PanelEmail:    owner.Email,
			PanelPassword: passForMail,
			SFTPUser:      res.Site.SFTPUser,
			SFTPPassword:  res.SFTPPassword,
			DBName:        res.Site.DBName,
			DBUser:        res.Site.DBUser,
			DBPassword:    res.DBPassword,
		})
	}
	return res, mailErr, nil
}

func (s *Server) ensureClientUser(email string) (*store.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.Store.GetUserByEmail(email)
	if err == nil {
		if u.Role != "client" {
			return nil, "", fmt.Errorf("email déjà utilisé par un compte non-client")
		}
		return u, "", nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, "", err
	}
	pass := randomPassword(8)
	hash, err := HashPassword(pass)
	if err != nil {
		return nil, "", err
	}
	name := strings.Split(email, "@")[0]
	u = &store.User{
		ID: uuid.NewString(), Email: email, PasswordHash: hash,
		Role: "client", DisplayName: name, CreatedAt: store.Now(),
	}
	if err := s.Store.CreateUser(*u); err != nil {
		return nil, "", err
	}
	return u, pass, nil
}

type stripeCheckoutSession struct {
	ID            string            `json:"id"`
	PaymentStatus string            `json:"payment_status"`
	CustomerEmail string            `json:"customer_email"`
	Metadata      map[string]string `json:"metadata"`
	CustomerDetails *struct {
		Email string `json:"email"`
	} `json:"customer_details"`
	CustomFields []struct {
		Key  string `json:"key"`
		Text *struct {
			Value string `json:"value"`
		} `json:"text"`
	} `json:"custom_fields"`
}

func (s stripeCheckoutSession) domain() (string, error) {
	if s.Metadata != nil {
		for _, k := range []string{"domain", "domaine"} {
			if v := strings.ToLower(strings.TrimSpace(s.Metadata[k])); v != "" {
				return v, nil
			}
		}
	}
	for _, f := range s.CustomFields {
		if f.Key == "domain" || f.Key == "domaine" {
			if f.Text != nil && strings.TrimSpace(f.Text.Value) != "" {
				return strings.ToLower(strings.TrimSpace(f.Text.Value)), nil
			}
		}
	}
	return "", fmt.Errorf("domaine absent (metadata.domain ou custom_fields)")
}

func (s stripeCheckoutSession) email() (string, error) {
	candidates := []string{s.CustomerEmail}
	if s.CustomerDetails != nil {
		candidates = append([]string{s.CustomerDetails.Email}, candidates...)
	}
	if s.Metadata != nil {
		candidates = append(candidates, s.Metadata["email"])
	}
	for _, e := range candidates {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" && strings.Contains(e, "@") {
			return e, nil
		}
	}
	return "", fmt.Errorf("email client absent")
}

func (s stripeCheckoutSession) appType() string {
	if s.Metadata != nil {
		t := strings.ToLower(strings.TrimSpace(s.Metadata["app_type"]))
		switch t {
		case "wordpress", "php", "static", "laravel", "prestashop":
			return t
		}
	}
	return "wordpress"
}

func verifyStripeSignature(payload []byte, header, secret string, tolerance time.Duration) error {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}
	if ts == "" || len(sigs) == 0 {
		return fmt.Errorf("en-tête Stripe-Signature incomplet")
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("timestamp invalide")
	}
	if tolerance > 0 && time.Since(time.Unix(sec, 0)).Abs() > tolerance {
		return fmt.Errorf("signature trop ancienne")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + "."))
	_, _ = mac.Write(payload)
	expect := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range sigs {
		if hmac.Equal([]byte(expect), []byte(sig)) {
			return nil
		}
	}
	return fmt.Errorf("aucune signature v1 valide")
}
