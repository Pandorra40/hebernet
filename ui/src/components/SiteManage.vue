<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'

const props = defineProps({
  id: String,
  admin: { type: Boolean, default: false },
})

const site = ref(null)
const quota = ref(null)
const logs = ref('')
const logKind = ref('access')
const cronJobs = ref([])
const cronForm = ref({ schedule: '0 * * * *', command: 'php -v' })
const error = ref('')
const notice = ref('')
const tab = ref('overview')
const busy = ref(false)

const base = computed(() => (props.admin ? '/admin/sites' : '/client/sites'))
const sftpHost = computed(() => window.location.hostname || 'votre-serveur')

const tabs = computed(() => {
  const t = [
    { id: 'overview', label: 'Aperçu' },
    { id: 'files', label: 'Fichiers' },
    { id: 'sftp', label: 'SFTP' },
    { id: 'ssl', label: 'SSL' },
    { id: 'bdd', label: 'Base de données' },
    { id: 'cron', label: 'Cron' },
    { id: 'quota', label: 'Quota' },
    { id: 'logs', label: 'Logs' },
  ]
  return t
})

async function load() {
  error.value = ''
  site.value = await api(`/api/sites/${props.id}`)
  try {
    quota.value = await api(`/api/sites/${props.id}/quota`)
  } catch {
    quota.value = null
  }
}

async function loadCron() {
  cronJobs.value = await api(`/api/sites/${props.id}/cron`)
}

async function loadLogs() {
  const res = await api(`/api/sites/${props.id}/logs?kind=${logKind.value}`)
  logs.value = res.lines || ''
}

async function ssl() {
  busy.value = true
  notice.value = ''
  try {
    site.value = await api(`/api/sites/${props.id}/ssl`, { method: 'POST' })
    notice.value = 'Certificat Let’s Encrypt demandé / activé.'
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function resetSftp() {
  busy.value = true
  notice.value = ''
  try {
    const res = await api(`/api/sites/${props.id}/sftp-reset`, { method: 'POST' })
    notice.value = `Nouveau mot de passe SFTP : ${res.sftp_password}`
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function resetDb() {
  busy.value = true
  notice.value = ''
  try {
    const res = await api(`/api/sites/${props.id}/db-reset`, { method: 'POST' })
    notice.value = `Nouveau mot de passe BDD : ${res.db_password}`
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function openAdminer() {
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    const res = await api(`/api/sites/${props.id}/adminer`, { method: 'POST' })
    notice.value = res.note || 'Adminer prêt.'
    window.open(res.url, '_blank', 'noopener')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function openFiles() {
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    const res = await api(`/api/sites/${props.id}/files`, { method: 'POST' })
    if (typeof res.quota_mb === 'number') {
      quota.value = {
        quota_mb: res.quota_mb,
        used_mb: res.used_mb ?? 0,
        percent: res.percent ?? 0,
      }
    }
    notice.value = res.note || `Root : ${res.root}`
    window.open(res.url, '_blank', 'noopener')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function addCron() {
  busy.value = true
  error.value = ''
  try {
    await api(`/api/sites/${props.id}/cron`, {
      method: 'POST',
      body: JSON.stringify(cronForm.value),
    })
    cronForm.value.command = ''
    await loadCron()
    notice.value = 'Tâche cron enregistrée (crontab synchronisée).'
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function delCron(id) {
  if (!confirm('Supprimer cette tâche cron ?')) return
  await api(`/api/sites/${props.id}/cron/${id}`, { method: 'DELETE' })
  await loadCron()
}

async function suspend() {
  site.value = await api(`/api/sites/${props.id}/suspend`, { method: 'POST' })
}
async function resume() {
  site.value = await api(`/api/sites/${props.id}/resume`, { method: 'POST' })
}
async function remove() {
  const ok = confirm(
    `Supprimer définitivement ${site.value?.domain} ?\n\n` +
      `Efface fichiers, SFTP, Nginx/PHP, SSL, BDD et crons.\nIrréversible.`
  )
  if (!ok) return
  await api(`/api/sites/${props.id}`, { method: 'DELETE' })
  location.href = '/admin/sites'
}

watch(tab, async (t) => {
  try {
    if (t === 'logs') await loadLogs()
    if (t === 'cron') await loadCron()
  } catch (e) {
    error.value = e.message
  }
})
watch(logKind, async () => {
  if (tab.value === 'logs') {
    try {
      await loadLogs()
    } catch (e) {
      error.value = e.message
    }
  }
})

onMounted(async () => {
  try {
    await load()
  } catch (e) {
    error.value = e.message
  }
})
</script>

<template>
  <div v-if="site">
    <p class="back"><RouterLink :to="base">← Sites</RouterLink></p>

    <header class="bandeau card">
      <div>
        <h1>{{ site.domain }}</h1>
        <p class="muted">
          {{ site.app_type }} · PHP {{ site.php_version }}
          · <span class="badge" :class="site.status">{{ site.status }}</span>
          · <span v-if="site.ssl_enabled" class="badge active">SSL</span>
          <span v-else class="badge">HTTP</span>
        </p>
      </div>
      <div class="meta">
        <div><span class="muted">User site</span><div class="mono">{{ site.sftp_user || site.linux_user }}</div></div>
        <div><span class="muted">Package</span><div>{{ site.package_name }}</div></div>
        <div v-if="quota"><span class="muted">Disque</span><div>{{ quota.used_mb }} / {{ quota.quota_mb }} Mo</div></div>
      </div>
      <div v-if="admin" class="actions">
        <button v-if="site.status === 'active'" class="secondary" type="button" @click="suspend">Suspendre</button>
        <button v-else-if="site.status === 'suspended'" type="button" @click="resume">Réactiver</button>
        <button class="danger" type="button" @click="remove">Supprimer</button>
      </div>
    </header>

    <div v-if="error" class="error-box">{{ error }}</div>
    <div v-if="notice" class="ok-box">{{ notice }}</div>

    <nav class="tabs">
      <button
        v-for="t in tabs"
        :key="t.id"
        type="button"
        class="secondary"
        :class="{ on: tab === t.id }"
        @click="tab = t.id"
      >{{ t.label }}</button>
    </nav>

    <div class="card panel">
      <template v-if="tab === 'overview'">
        <h2>Paramètres du site</h2>
        <dl class="kv">
          <div><dt>Domaine</dt><dd class="mono">{{ site.domain }}</dd></div>
          <div><dt>Type</dt><dd>{{ site.app_type }}</dd></div>
          <div><dt>PHP</dt><dd class="mono">{{ site.php_version }}</dd></div>
          <div><dt>Répertoire</dt><dd class="mono">{{ site.home_path }}/public_html</dd></div>
          <div v-if="admin"><dt>Client</dt><dd>{{ site.owner_email }}</dd></div>
        </dl>
      </template>

      <template v-else-if="tab === 'files'">
        <h2>Gestionnaire de fichiers</h2>
        <p class="muted">FileBrowser Quantum — limité au home du site. SFTP reste disponible.</p>
        <div v-if="quota" class="quota-banner">
          <div>
            <span class="muted">Quota disque du site</span>
            <strong>{{ quota.used_mb }} / {{ quota.quota_mb }} Mo</strong>
          </div>
          <div class="bar"><span :style="{ width: Math.min(100, quota.percent || 0) + '%' }" /></div>
          <p class="muted tiny">Appliqué via setquota (package). L’affichage disque de Quantum reflète l’hôte tant que limitBytes (v2.1+) n’est pas disponible.</p>
        </div>
        <button type="button" :disabled="busy" @click="openFiles">Ouvrir les fichiers</button>
      </template>

      <template v-else-if="tab === 'sftp'">
        <h2>Accès SFTP</h2>
        <dl class="kv">
          <div><dt>Hôte</dt><dd class="mono">{{ sftpHost }}</dd></div>
          <div><dt>Port</dt><dd class="mono">22</dd></div>
          <div><dt>Utilisateur</dt><dd class="mono">{{ site.sftp_user }}</dd></div>
        </dl>
        <button type="button" :disabled="busy" @click="resetSftp">Régénérer le mot de passe</button>
      </template>

      <template v-else-if="tab === 'ssl'">
        <h2>SSL / Let’s Encrypt</h2>
        <p>État : <strong>{{ site.ssl_enabled ? 'actif' : 'non configuré' }}</strong></p>
        <button type="button" :disabled="busy" @click="ssl">
          {{ site.ssl_enabled ? 'Renouveler' : 'Activer Let’s Encrypt' }}
        </button>
      </template>

      <template v-else-if="tab === 'bdd'">
        <h2>Base de données</h2>
        <template v-if="site.db_name">
          <dl class="kv">
            <div><dt>Nom</dt><dd class="mono">{{ site.db_name }}</dd></div>
            <div><dt>Utilisateur</dt><dd class="mono">{{ site.db_user }}</dd></div>
            <div><dt>Hôte</dt><dd class="mono">localhost</dd></div>
          </dl>
          <div class="row-actions">
            <button type="button" :disabled="busy" @click="openAdminer">Ouvrir Adminer</button>
            <button type="button" class="secondary" :disabled="busy" @click="resetDb">Régénérer le mot de passe</button>
          </div>
        </template>
        <p v-else class="muted">Pas de base (site statique). Créez un site WordPress/PHP pour Adminer.</p>
      </template>

      <template v-else-if="tab === 'cron'">
        <h2>Tâches planifiées</h2>
        <div class="cron-form">
          <div class="field"><label>Schedule (crontab)</label><input v-model="cronForm.schedule" placeholder="0 * * * *" class="mono" /></div>
          <div class="field"><label>Commande</label><input v-model="cronForm.command" placeholder="cd ~/public_html && php cron.php" class="mono" /></div>
          <button type="button" :disabled="busy || !cronForm.command" @click="addCron">Ajouter</button>
        </div>
        <table>
          <thead><tr><th>Schedule</th><th>Commande</th><th></th></tr></thead>
          <tbody>
            <tr v-if="!cronJobs.length"><td colspan="3" class="muted">Aucune tâche.</td></tr>
            <tr v-for="j in cronJobs" :key="j.id">
              <td class="mono">{{ j.schedule }}</td>
              <td class="mono">{{ j.command }}</td>
              <td><button type="button" class="danger compact" @click="delCron(j.id)">Suppr.</button></td>
            </tr>
          </tbody>
        </table>
      </template>

      <template v-else-if="tab === 'quota'">
        <h2>Quota disque</h2>
        <p v-if="quota"><strong>{{ quota.used_mb }} / {{ quota.quota_mb }} Mo</strong></p>
        <div v-if="quota" class="bar"><span :style="{ width: Math.min(100, quota.percent || 0) + '%' }" /></div>
      </template>

      <template v-else-if="tab === 'logs'">
        <h2>Logs</h2>
        <div class="log-kinds">
          <button type="button" class="secondary" :class="{ on: logKind === 'access' }" @click="logKind = 'access'">Accès</button>
          <button type="button" class="secondary" :class="{ on: logKind === 'error' }" @click="logKind = 'error'">Erreurs</button>
          <button type="button" class="secondary" @click="loadLogs">Actualiser</button>
        </div>
        <pre class="log mono">{{ logs || '…' }}</pre>
      </template>
    </div>
  </div>
  <div v-else-if="error" class="error-box">{{ error }}</div>
  <p v-else class="muted">Chargement…</p>
</template>

<style scoped>
.back { margin-bottom: 0.75rem; }
.bandeau {
  display: flex; justify-content: space-between; gap: 1rem; flex-wrap: wrap;
  margin-bottom: 1rem; align-items: flex-start;
}
.bandeau h1 { margin: 0 0 0.35rem; font-size: 1.55rem; }
.meta { display: flex; gap: 1.25rem; flex-wrap: wrap; }
.meta .muted { font-size: 0.78rem; display: block; margin-bottom: 0.15rem; }
.actions { display: flex; gap: 0.4rem; flex-wrap: wrap; }
.tabs { display: flex; flex-wrap: wrap; gap: 0.4rem; margin: 0 0 1rem; }
.tabs button.on, .log-kinds button.on { border-color: var(--accent); color: var(--accent-hover); }
.panel h2 { margin: 0 0 1rem; font-size: 1.15rem; }
.kv { display: grid; gap: 0.75rem; margin: 0 0 1rem; }
.kv > div { display: grid; grid-template-columns: 140px 1fr; gap: 0.5rem; }
.kv dt { margin: 0; color: var(--muted); }
.kv dd { margin: 0; }
.row-actions, .cron-form { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: flex-end; margin-bottom: 1rem; }
.cron-form .field { flex: 1; min-width: 180px; margin: 0; }
.bar { height: 10px; background: var(--bg-soft); border-radius: 99px; overflow: hidden; margin: 0.75rem 0; }
.bar span { display: block; height: 100%; background: var(--accent); }
.quota-banner {
  margin: 0 0 1rem; padding: 0.85rem 1rem;
  border: 1px solid var(--border); border-radius: 8px; background: var(--bg-soft);
}
.quota-banner strong { display: block; margin-top: 0.15rem; }
.quota-banner .bar { margin: 0.55rem 0 0.35rem; }
.tiny { font-size: 0.78rem; margin: 0; }
.log-kinds { display: flex; gap: 0.4rem; margin-bottom: 0.75rem; }
.log {
  background: var(--bg); border: 1px solid var(--border); border-radius: 8px;
  padding: 0.85rem; max-height: 360px; overflow: auto; font-size: 0.8rem; white-space: pre-wrap;
}
button.compact { padding: 0.3rem 0.55rem; font-size: 0.8rem; }
</style>
