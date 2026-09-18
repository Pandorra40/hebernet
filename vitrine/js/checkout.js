/* Checkout / activation — vitrine Hébernet */
(function () {
  // Priorité : window.HEBERNET_API → localhost:8787 → même origine (vitrine :8088 proxifie /api)
  const API = (function () {
    if (window.HEBERNET_API) return String(window.HEBERNET_API).replace(/\/$/, '')
    const h = location.hostname
    if (h === '127.0.0.1' || h === 'localhost') return 'http://127.0.0.1:8787'
    return location.origin
  })()

  const params = new URLSearchParams(location.search)
  const sessionId = params.get('session_id')
  const cancelled = params.get('annule') === '1' || params.get('checkout') === 'annule'

  const vueForm = document.getElementById('vue-form')
  const vueStatus = document.getElementById('vue-status')
  const vueCancel = document.getElementById('vue-cancel')

  if (cancelled) {
    vueForm.hidden = true
    vueStatus.hidden = true
    vueCancel.hidden = false
    return
  }

  if (sessionId) {
    vueForm.hidden = true
    vueCancel.hidden = true
    vueStatus.hidden = false
    pollStatus(sessionId)
    return
  }

  const form = document.getElementById('checkout-form')
  const errBox = document.getElementById('form-error')
  const btn = document.getElementById('pay-btn')

  form.addEventListener('submit', async (e) => {
    e.preventDefault()
    errBox.hidden = true
    btn.disabled = true
    btn.textContent = 'Redirection…'
    try {
      const res = await fetch(API + '/api/stripe/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: form.domain.value.trim(),
          email: form.email.value.trim(),
          app_type: form.app_type.value,
        }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'Erreur ' + res.status)
      if (!data.url) throw new Error('URL de paiement manquante')
      location.href = data.url
    } catch (err) {
      let msg = err.message || String(err)
      if (msg === 'Failed to fetch' || msg.includes('NetworkError') || msg.includes('Load failed')) {
        msg = 'Impossible de joindre l’API Hébernet (' + API + '). Sur la VM, le panneau doit tourner sur le port 80 (/api).'
      }
      errBox.textContent = msg
      errBox.hidden = false
      btn.disabled = false
      btn.textContent = 'Payer — 49 €'
    }
  })

  const HINTS = [
    'Création du compte…',
    'Préparation de l’espace disque…',
    'Configuration du serveur web…',
    'Derniers réglages…',
  ]

  function setProgress(pct) {
    const bar = document.getElementById('status-bar')
    const el = document.getElementById('status-pct')
    const progress = document.getElementById('progress')
    const n = Math.max(0, Math.min(100, Math.round(pct)))
    bar.style.width = n + '%'
    el.textContent = n + ' %'
    progress.setAttribute('aria-valuenow', String(n))
  }

  function setSteps(phase) {
    // phase: pay | prep | ready | error
    const pay = document.getElementById('step-pay')
    const prep = document.getElementById('step-prep')
    const ready = document.getElementById('step-ready')
    ;[pay, prep, ready].forEach((el) => el.classList.remove('is-done', 'is-current', 'is-error'))
    if (phase === 'error') {
      pay.classList.add('is-done')
      prep.classList.add('is-error')
      return
    }
    pay.classList.add('is-done')
    if (phase === 'pay' || phase === 'prep') {
      prep.classList.add('is-current')
    } else if (phase === 'ready') {
      prep.classList.add('is-done')
      ready.classList.add('is-done', 'is-current')
    }
  }

  function setIcon(kind) {
    const icon = document.getElementById('status-icon')
    const card = document.getElementById('status-card')
    card.classList.remove('is-ready', 'is-error')
    if (kind === 'ready') {
      card.classList.add('is-ready')
      icon.innerHTML = '<span class="status-check" aria-hidden="true"></span>'
    } else if (kind === 'error') {
      card.classList.add('is-error')
      icon.innerHTML = '<span class="status-cross" aria-hidden="true"></span>'
    } else {
      icon.innerHTML = '<span class="status-spinner" aria-hidden="true"></span>'
    }
  }

  async function pollStatus(id) {
    const label = document.getElementById('status-label')
    const detail = document.getElementById('status-detail')
    const hint = document.getElementById('status-hint')
    const intro = document.getElementById('status-intro')
    const actions = document.getElementById('status-actions')
    const panelLink = document.getElementById('panel-link')
    let tries = 0
    let displayPct = 12

    const easeToward = (target) => {
      displayPct += (target - displayPct) * 0.35
      if (Math.abs(target - displayPct) < 0.5) displayPct = target
      setProgress(displayPct)
    }

    const tick = async () => {
      tries++
      try {
        const res = await fetch(API + '/api/stripe/status/' + encodeURIComponent(id))
        const data = await res.json()
        const st = data.status || 'unknown'
        const domain = data.domain || ''

        if (st === 'active') {
          setSteps('ready')
          setIcon('ready')
          easeToward(100)
          setProgress(100)
          label.textContent = 'Hébergement prêt'
          detail.textContent = domain
            ? domain + ' est actif. Les identifiants sont dans votre boîte mail (si le mail est configuré).'
            : 'Votre espace est actif. Les identifiants sont dans votre boîte mail (si le mail est configuré).'
          hint.hidden = true
          intro.textContent = 'Tout est bon.'
          if (data.panel_url) panelLink.href = data.panel_url
          actions.hidden = false
          return
        }
        if (st === 'error') {
          setSteps('error')
          setIcon('error')
          setProgress(100)
          label.textContent = 'Activation en erreur'
          detail.textContent =
            data.message || 'Contactez le support avec l’e-mail utilisé pour payer.'
          hint.hidden = true
          actions.hidden = false
          panelLink.hidden = true
          // Auto-retry a few times (agent may come back)
          if (tries < 8) {
            setTimeout(tick, 3000)
            label.textContent = 'Nouvelle tentative…'
            setIcon('spin')
            setSteps('prep')
            setProgress(Math.min(85, 40 + tries * 5))
            hint.hidden = false
            hint.textContent = 'On réessaie automatiquement.'
            actions.hidden = true
            panelLink.hidden = false
            return
          }
          return
        }

        setSteps('prep')
        setIcon('spin')
        const target = st === 'provisioning' ? Math.min(92, 45 + tries * 6) : Math.min(78, 18 + tries * 5)
        easeToward(target)
        label.textContent = HINTS[Math.min(HINTS.length - 1, Math.floor((tries - 1) / 2))]
        detail.textContent = domain ? 'Site : ' + domain : data.message || 'Synchronisation…'
        hint.hidden = false
      } catch {
        setSteps('prep')
        label.textContent = 'Connexion à l’API…'
        detail.textContent = 'Nouvelle tentative dans un instant.'
        easeToward(Math.min(40, 10 + tries * 2))
      }

      if (tries < 60) {
        setTimeout(tick, tries < 8 ? 1500 : 2500)
      } else {
        setSteps('prep')
        setIcon('spin')
        label.textContent = 'Encore un instant…'
        detail.textContent =
          'L’activation continue en arrière-plan. Rechargez cette page dans une minute, ou ouvrez votre boîte mail.'
        hint.textContent = 'Vous pouvez aussi ouvrir le panneau si le compte a déjà été créé.'
        actions.hidden = false
      }
    }
    tick()
  }
})()
