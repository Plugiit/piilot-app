package gateway

// maintenancePage est servie aux navigations quand aucune instance ne repond.
//
// Autonome — ni police, ni feuille de style, ni script externes : l'application
// qui les servirait est justement absente. Elle interroge la passerelle toutes
// les deux secondes et se recharge des qu'une instance peut servir ; sans
// JavaScript, la balise refresh prend le relais.
const maintenancePage = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="15">
<meta name="robots" content="noindex">
<title>Mise à jour en cours · Piilot</title>
<style>
  :root { color-scheme: light; }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100dvh; display: grid; place-items: center; padding: 16px;
    background: #f5f5f5; color: #1b1b1b;
    font: 14px/1.5 ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
  }
  main {
    width: 100%; max-width: 380px; padding: 24px;
    background: #fff; border: 1px solid #e4e4e4; border-radius: 12px;
  }
  h1 { margin: 0 0 4px; font-size: 16px; font-weight: 500; display: flex; align-items: center; gap: 8px; }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: #ff782b; animation: pulse 1.4s ease-in-out infinite; }
  p { margin: 0; color: #73757c; font-size: 13px; }
  @keyframes pulse { 50% { opacity: .3; } }
  @media (prefers-reduced-motion: reduce) { .dot { animation: none; } }
</style>
</head>
<body>
<main role="status">
  <h1><span class="dot" aria-hidden="true"></span>Mise à jour en cours</h1>
  <p>Piilot revient dans un instant. Cette page se rechargera d’elle-même.</p>
</main>
<script>
  (function poll() {
    fetch('/health/gateway', { cache: 'no-store' })
      .then(function (r) { return r.json() })
      .then(function (s) { if (s.ready) location.reload(); else setTimeout(poll, 2000) })
      .catch(function () { setTimeout(poll, 2000) })
  })()
</script>
</body>
</html>
`
