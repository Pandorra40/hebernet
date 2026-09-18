# Stripe → Checkout → provision → suivi

## Parcours client

1. Vitrine `/paiement.html` — domaine + e-mail  
2. `POST /api/stripe/checkout` → URL Stripe Checkout (ou démo locale)  
3. Paiement (`4242…` en test)  
4. Retour `paiement.html?session_id=…` — suivi *en cours* / *prêt*  
5. Webhook `checkout.session.completed` → compte + site + mail Resend  

## Variables API

| Variable | Rôle |
|---|---|
| `STRIPE_SECRET_KEY` | `sk_test_…` — crée les Checkout Sessions |
| `STRIPE_WEBHOOK_SECRET` | `whsec_…` |
| `HEBERNET_CHECKOUT_DEMO` | `1` (défaut) si pas de clé : provision immédiat sans Stripe |
| `HEBERNET_PRICE_CENTS` | défaut `100` (1 € test) ; `4900` = 49 € |
| `HEBERNET_VITRINE_URL` | `http://127.0.0.1:8088` (success/cancel) |
| `HEBERNET_STRIPE_PACKAGE` | `Starter` |
| `RESEND_API_KEY` / `HEBERNET_MAIL_FROM` | mail de bienvenue |
| `HEBERNET_PANEL_URL` | lien panneau dans le suivi + mail |

## Test local (sans Stripe)

```bash
# API avec HEBERNET_CHECKOUT_DEMO=1 (défaut si pas de STRIPE_SECRET_KEY)
# Vitrine : http://127.0.0.1:8088/paiement.html
# → formulaire → redirection directe vers ?session_id=cs_demo_… → statut « prêt »
```

## Test avec Stripe (4242)

```bash
export STRIPE_SECRET_KEY=sk_test_…
export STRIPE_WEBHOOK_SECRET=whsec_…   # stripe listen
export HEBERNET_CHECKOUT_DEMO=0
export HEBERNET_PRICE_CENTS=100
export HEBERNET_VITRINE_URL=http://127.0.0.1:8088

stripe listen --events checkout.session.completed --forward-to 127.0.0.1:8787/api/stripe/webhook
# puis ouvrir paiement.html, domaine bidon, payer 4242
```
