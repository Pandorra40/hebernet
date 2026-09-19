<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { api } from '../../api'

const router = useRouter()
const sites = ref([])
const packages = ref([])
const users = ref([])
const error = ref('')
const notice = ref('')
const showForm = ref(false)
const form = ref({ domain: '', app_type: 'static', owner_id: '', package_id: '' })
const createdCred = ref(null)
const loading = ref(false)
const busyId = ref('')

async function load() {
  error.value = ''
  try {
    ;[sites.value, packages.value, users.value] = await Promise.all([
      api('/api/sites'),
      api('/api/packages'),
      api('/api/users'),
    ])
    const clients = users.value.filter((u) => u.role === 'client')
    if (!form.value.owner_id && clients[0]) form.value.owner_id = clients[0].id
    if (!form.value.package_id && packages.value[0]) form.value.package_id = packages.value[0].id
  } catch (e) {
    error.value = e.message
  }
}

async function createSite() {
  loading.value = true
  error.value = ''
  createdCred.value = null
  try {
    const res = await api('/api/sites', { method: 'POST', body: JSON.stringify(form.value) })
    createdCred.value = res
    showForm.value = false
    form.value.domain = ''
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function suspend(s) {
  busyId.value = s.id
  try {
    await api(`/api/sites/${s.id}/suspend`, { method: 'POST' })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = ''
  }
}

async function resume(s) {
  busyId.value = s.id
  try {
    await api(`/api/sites/${s.id}/resume`, { method: 'POST' })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = ''
  }
}

async function remove(s) {
  const ok = confirm(
    `Supprimer définitivement « ${s.domain} » ?\n\n` +
    `Cette action efface en une fois : fichiers, utilisateur SFTP, config Nginx/PHP-FPM, certificat SSL, et la base MariaDB associée (si présente).\n\n` +
    `Irréversible.`
  )
  if (!ok) return
  busyId.value = s.id
  error.value = ''
  notice.value = ''
  try {
    const res = await api(`/api/sites/${s.id}`, { method: 'DELETE' })
    notice.value = res.message || `Site ${s.domain} supprimé.`
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = ''
  }
}

onMounted(load)
</script>

<template>
  <div>
    <header class="head">
      <div>
        <h1>Sites</h1>
        <p class="muted">Tous les sites de l’hôte</p>
      </div>
      <button type="button" @click="showForm = !showForm">
        {{ showForm ? 'Annuler' : 'Ajouter un site' }}
      </button>
    </header>

    <div v-if="error" class="error-box">{{ error }}</div>
    <div v-if="notice" class="ok-box">{{ notice }}</div>
    <div v-if="createdCred" class="ok-box">
      Site <strong>{{ createdCred.site?.domain }}</strong> créé.
      SFTP <span class="mono">{{ createdCred.site?.sftp_user }}</span>
      / <span class="mono">{{ createdCred.sftp_password }}</span>
      (affiché une seule fois)
    </div>

    <div v-if="showForm" class="card form">
      <div class="grid">
        <div class="field">
          <label>Domaine</label>
          <input v-model="form.domain" placeholder="exemple.fr" required />
        </div>
        <div class="field">
          <label>Type</label>
          <select v-model="form.app_type">
            <option value="static">Statique</option>
            <option value="hugo">Hugo</option>
            <option value="php">PHP</option>
            <option value="bludit">Bludit</option>
            <option value="codeigniter">CodeIgniter</option>
          </select>
        </div>
        <div class="field">
          <label>Client</label>
          <select v-model="form.owner_id">
            <option v-for="u in users.filter(x => x.role === 'client')" :key="u.id" :value="u.id">
              {{ u.display_name || u.email }}
            </option>
          </select>
        </div>
        <div class="field">
          <label>Package</label>
          <select v-model="form.package_id">
            <option v-for="p in packages" :key="p.id" :value="p.id">
              {{ p.name }} ({{ p.disk_mb }} Mo)
            </option>
          </select>
        </div>
      </div>
      <button type="button" :disabled="loading || !form.domain" @click="createSite">
        {{ loading ? 'Provision…' : 'Créer' }}
      </button>
    </div>

    <div class="card table-wrap">
      <table>
        <thead>
          <tr>
            <th>Domaine</th>
            <th>Type</th>
            <th>Client</th>
            <th>Package</th>
            <th>Statut</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!sites.length">
            <td colspan="6" class="muted">Aucun site pour l’instant.</td>
          </tr>
          <tr v-for="s in sites" :key="s.id">
            <td><RouterLink :to="`/admin/sites/${s.id}`">{{ s.domain }}</RouterLink></td>
            <td>{{ s.app_type }}</td>
            <td>{{ s.owner_email }}</td>
            <td>{{ s.package_name }}</td>
            <td><span class="badge" :class="s.status">{{ s.status }}</span></td>
            <td class="actions">
              <button type="button" class="secondary compact" @click="router.push(`/admin/sites/${s.id}`)">Gérer</button>
              <button
                v-if="s.status === 'active'"
                type="button"
                class="secondary compact"
                :disabled="busyId === s.id"
                @click="suspend(s)"
              >Suspendre</button>
              <button
                v-else-if="s.status === 'suspended'"
                type="button"
                class="compact"
                :disabled="busyId === s.id"
                @click="resume(s)"
              >Réactiver</button>
              <button
                type="button"
                class="danger compact"
                :disabled="busyId === s.id"
                @click="remove(s)"
              >Supprimer</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.25rem; gap: 1rem; }
h1 { margin: 0 0 0.25rem; font-size: 1.6rem; }
.form { margin-bottom: 1rem; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem 1rem; }
.table-wrap { padding: 0.5rem 1rem; overflow-x: auto; }
.actions { display: flex; flex-wrap: wrap; gap: 0.35rem; }
button.compact { padding: 0.35rem 0.65rem; font-size: 0.85rem; }
@media (max-width: 700px) { .grid { grid-template-columns: 1fr; } }
</style>
