package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/hebernet/hebernet/internal/agentclient"
	"github.com/hebernet/hebernet/internal/api"
	"github.com/hebernet/hebernet/internal/mail"
	"github.com/hebernet/hebernet/internal/store"
	"github.com/hebernet/hebernet/internal/tools"
)

func main() {
	addr := flag.String("addr", envOr("HEBERNET_API_ADDR", "127.0.0.1:8787"), "listen address")
	dbPath := flag.String("db", envOr("HEBERNET_DB", "data/hebernet.db"), "sqlite path")
	socket := flag.String("socket", envOr("HEBERNET_AGENT_SOCKET", "data/agent.sock"), "agent unix socket")
	dataDir := flag.String("data", envOr("HEBERNET_DATA", "data"), "data dir (demo homes)")
	seed := flag.Bool("seed", true, "seed demo admin/client + packages + site")
	flag.Parse()

	absDB, _ := filepath.Abs(*dbPath)
	absSock, _ := filepath.Abs(*socket)
	absData, _ := filepath.Abs(*dataDir)
	root, _ := filepath.Abs(".")

	st, err := store.Open(absDB)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if *seed {
		if err := seedDemo(st, absData); err != nil {
			log.Printf("seed: %v", err)
		}
	}

	ag := &agentclient.Client{SocketPath: absSock}
	tm := &tools.Manager{
		Root:       root,
		DataDir:    absData,
		ListenAddr: envOr("HEBERNET_SIDECAR_BIND", "0.0.0.0"),
	}
	srv := api.New(st, ag, tm)
	srv.StripeWebhookSecret = envOr("STRIPE_WEBHOOK_SECRET", "")
	srv.StripeSecretKey = envOr("STRIPE_SECRET_KEY", "")
	srv.StripePackageName = envOr("HEBERNET_STRIPE_PACKAGE", "Starter")
	srv.StripePriceCents = envInt("HEBERNET_PRICE_CENTS", 100)
	srv.StripeCheckoutDemo = envOr("HEBERNET_CHECKOUT_DEMO", "1") != "0"
	srv.VitrineURL = envOr("HEBERNET_VITRINE_URL", "http://127.0.0.1:8088")
	panelURL := envOr("HEBERNET_PANEL_URL", "http://127.0.0.1")
	srv.PublicHost = api.PublicHostFromURL(panelURL)
	if v := envOr("HEBERNET_PUBLIC_HOST", ""); v != "" {
		srv.PublicHost = v
	}
	srv.Mail = mail.Config{
		APIKey:   envOr("RESEND_API_KEY", ""),
		From:     envOr("HEBERNET_MAIL_FROM", ""),
		ReplyTo:  envOr("HEBERNET_MAIL_REPLY_TO", ""),
		PanelURL: panelURL,
		ServerIP: envOr("HEBERNET_SERVER_IP", ""),
	}
	if srv.StripeWebhookSecret != "" {
		log.Printf("stripe webhook enabled → POST /api/stripe/webhook (package %q)", srv.StripePackageName)
	}
	if srv.StripeSecretKey != "" {
		log.Printf("stripe checkout enabled → POST /api/stripe/checkout (%d cents)", srv.StripePriceCents)
	} else if srv.StripeCheckoutDemo {
		log.Printf("stripe checkout DEMO (no STRIPE_SECRET_KEY) → provision immédiat")
	}
	if srv.Mail.Enabled() {
		log.Printf("welcome mail via Resend enabled")
	}
	log.Printf("sidecars (files/adminer) → http://%s:<port> (bind %s)", srv.PublicHost, tm.ListenAddr)
	log.Printf("api listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}

func seedDemo(st *store.Store, dataDir string) error {
	admin, err := st.GetUserByEmail("admin@hebernet.local")
	if err != nil {
		adminHash, err := api.HashPassword("admin")
		if err != nil {
			return err
		}
		admin = &store.User{
			ID: uuid.NewString(), Email: "admin@hebernet.local", PasswordHash: adminHash,
			Role: "admin", DisplayName: "Administrateur", CreatedAt: store.Now(),
		}
		if err := st.CreateUser(*admin); err != nil {
			return err
		}
	}

	client, err := st.GetUserByEmail("client@hebernet.local")
	if err != nil {
		clientHash, err := api.HashPassword("client")
		if err != nil {
			return err
		}
		client = &store.User{
			ID: uuid.NewString(), Email: "client@hebernet.local", PasswordHash: clientHash,
			Role: "client", DisplayName: "Client démo", CreatedAt: store.Now(),
		}
		if err := st.CreateUser(*client); err != nil {
			return err
		}
	}

	pkgs, err := st.ListPackages()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		pkgs = []store.Package{
			{ID: uuid.NewString(), Name: "Starter", DiskMB: 5120, MaxSites: 1, MaxDatabases: 1, Description: "5 Go — 1 site — 1 BDD", CreatedAt: store.Now()},
			{ID: uuid.NewString(), Name: "Pro", DiskMB: 20480, MaxSites: 5, MaxDatabases: 5, Description: "20 Go — 5 sites — 5 BDD", CreatedAt: store.Now()},
			{ID: uuid.NewString(), Name: "Business", DiskMB: 51200, MaxSites: 20, MaxDatabases: 20, Description: "50 Go — 20 sites — 20 BDD", CreatedAt: store.Now()},
		}
		for _, p := range pkgs {
			if err := st.CreatePackage(p); err != nil {
				return err
			}
		}
	}

	owned, err := st.ListSitesByOwner(client.ID)
	if err != nil {
		return err
	}
	hasWP := false
	for _, s := range owned {
		if s.AppType == "wordpress" || s.AppType == "php" {
			hasWP = true
			break
		}
	}
	if !hasWP {
		linuxUser := "hb_demo_wp"
		home := filepath.Join(dataDir, "homes", linuxUser)
		public := filepath.Join(home, "public_html")
		_ = os.MkdirAll(public, 0o755)
		_ = os.WriteFile(filepath.Join(public, "index.php"), []byte("<?php echo 'WP démo Hébernet';\n"), 0o644)
		now := store.Now()
		site := store.Site{
			ID: uuid.NewString(), Domain: "demo-wp.client.test", AppType: "wordpress",
			OwnerID: client.ID, PackageID: pkgs[0].ID, LinuxUser: linuxUser, HomePath: home,
			PHPVersion: "8.5", Status: "active", SSLEnabled: false,
			DBName: "hb_demo_wp", DBUser: "hb_demo_wp", SFTPUser: linuxUser,
			QuotaMB: pkgs[0].DiskMB, QuotaUsedMB: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := st.CreateSite(site); err != nil {
			return err
		}
		log.Printf("seed site WP démo : demo-wp.client.test (Adminer / cron / FM)")
	}

	log.Printf("seed OK — admin@hebernet.local / admin · client@hebernet.local / client")
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
