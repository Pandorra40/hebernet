#!/usr/bin/env php
<?php
declare(strict_types=1);

/*
 * Crée une session Stripe Checkout de test avec metadata.domain → webhook Hébernet.
 *
 *   STRIPE_SECRET_KEY='sk_test_...' \
 *     php scripts/stripe-checkout-cli.php domaine-test.fr client@email.fr
 */

const STRIPE_SESSIONS_URL = 'https://api.stripe.com/v1/checkout/sessions';

function secret_stripe(): string
{
    $depuisEnv = getenv('STRIPE_SECRET_KEY');
    if (is_string($depuisEnv) && $depuisEnv !== '') {
        return $depuisEnv;
    }
    throw new RuntimeException('Exporter STRIPE_SECRET_KEY=sk_test_...');
}

if (PHP_SAPI !== 'cli') {
    exit("CLI uniquement.\n");
}

$domaine = strtolower($argv[1] ?? '');
$email   = $argv[2] ?? '';

if ($domaine === '' || !preg_match('/^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}$/i', $domaine)) {
    exit("Usage : STRIPE_SECRET_KEY='sk_test_...' php scripts/stripe-checkout-cli.php domaine.fr client@email.fr\n");
}
if (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
    exit("Email client invalide.\n");
}

$success = getenv('HEBERNET_VITRINE_SUCCESS') ?: 'http://127.0.0.1:8088/?checkout=ok';
$cancel  = getenv('HEBERNET_VITRINE_CANCEL') ?: 'http://127.0.0.1:8088/?checkout=annule';
$amount  = getenv('HEBERNET_CHECKOUT_CENTS') ?: '100';
$appType = getenv('HEBERNET_APP_TYPE') ?: 'wordpress';

$champs = [
    'mode'                                   => 'payment',
    'success_url'                            => $success,
    'cancel_url'                             => $cancel,
    'customer_email'                         => $email,
    'metadata[domain]'                       => $domaine,
    'metadata[app_type]'                     => $appType,
    'line_items[0][quantity]'                => '1',
    'line_items[0][price_data][currency]'    => 'eur',
    'line_items[0][price_data][unit_amount]' => $amount,
    'line_items[0][price_data][product_data][name]' => 'Hébernet — ' . $domaine,
];

$ch = curl_init(STRIPE_SESSIONS_URL);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_POST           => true,
    CURLOPT_USERPWD        => secret_stripe() . ':',
    CURLOPT_POSTFIELDS     => http_build_query($champs),
    CURLOPT_TIMEOUT        => 30,
]);
$brut = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($brut === false) {
    exit("Réseau Stripe : échec\n");
}
$json = json_decode($brut, true);
if ($code < 200 || $code >= 300) {
    fwrite(STDERR, "HTTP $code\n$brut\n");
    exit(1);
}

echo "Session : " . ($json['id'] ?? '?') . "\n";
echo "URL     : " . ($json['url'] ?? '') . "\n";
echo "Après paiement, le webhook Hébernet POST /api/stripe/webhook provisionne le site.\n";
