<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'

const packages = ref([])
const error = ref('')
const notice = ref('')
const editingId = ref('')
const form = ref({ name: '', disk_mb: 10240, max_sites: 1, max_databases: 1, description: '' })

async function load() {
  packages.value = await api('/api/packages')
}

function startEdit(p) {
  editingId.value = p.id
  form.value = {
    name: p.name,
    disk_mb: p.disk_mb,
    max_sites: p.max_sites,
    max_databases: p.max_databases ?? 1,
    description: p.description || '',
  }
}

function cancelEdit() {
  editingId.value = ''
  form.value = { name: '', disk_mb: 10240, max_sites: 1, max_databases: 1, description: '' }
}

async function savePkg() {
  error.value = ''
  notice.value = ''
  const payload = {
    name: form.value.name,
    disk_mb: Number(form.value.disk_mb),
    max_sites: Number(form.value.max_sites),
    max_databases: Number(form.value.max_databases),
    description: form.value.description,
  }
  try {
    if (editingId.value) {
      await api(`/api/packages/${editingId.value}`, { method: 'PUT', body: JSON.stringify(payload) })
      notice.value = 'Offre mise à jour.'
    } else {
      await api('/api/packages', { method: 'POST', body: JSON.stringify(payload) })
      notice.value = 'Offre créée.'
    }
    cancelEdit()
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function removePkg(p) {
  if (!confirm(`Supprimer l’offre « ${p.name} » ?`)) return
  error.value = ''
  try {
    await api(`/api/packages/${p.id}`, { method: 'DELETE' })
    if (editingId.value === p.id) cancelEdit()
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  try { await load() } catch (e) { error.value = e.message }
})
</script>

<template>
  <div>
    <header class="head">
      <div>
        <h1>Packages</h1>
        <p class="muted">Personnalisez disque, sites et bases de données</p>
      </div>
    </header>
    <div v-if="error" class="error-box">{{ error }}</div>
    <div v-if="notice" class="ok-box">{{ notice }}</div>

    <div class="card form">
      <h2>{{ editingId ? 'Modifier l’offre' : 'Nouvelle offre' }}</h2>
      <div class="grid">
        <div class="field"><label>Nom</label><input v-model="form.name" /></div>
        <div class="field"><label>Disque (Mo)</label><input v-model="form.disk_mb" type="number" min="100" /></div>
        <div class="field"><label>Sites max</label><input v-model="form.max_sites" type="number" min="1" /></div>
        <div class="field"><label>Bases de données max</label><input v-model="form.max_databases" type="number" min="0" /></div>
        <div class="field wide"><label>Description</label><input v-model="form.description" /></div>
      </div>
      <div class="row">
        <button type="button" :disabled="!form.name" @click="savePkg">
          {{ editingId ? 'Enregistrer' : 'Ajouter' }}
        </button>
        <button v-if="editingId" type="button" class="secondary" @click="cancelEdit">Annuler</button>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>Nom</th>
            <th>Disque</th>
            <th>Sites</th>
            <th>BDD</th>
            <th>Description</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in packages" :key="p.id">
            <td>{{ p.name }}</td>
            <td>{{ p.disk_mb }} Mo</td>
            <td>{{ p.max_sites }}</td>
            <td>{{ p.max_databases }}</td>
            <td class="muted">{{ p.description }}</td>
            <td class="actions">
              <button type="button" class="secondary compact" @click="startEdit(p)">Modifier</button>
              <button type="button" class="danger compact" @click="removePkg(p)">Supprimer</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.head { margin-bottom: 1.25rem; }
h1 { margin: 0 0 0.25rem; }
h2 { margin: 0 0 1rem; font-size: 1.05rem; }
.form { margin-bottom: 1rem; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.field.wide { grid-column: 1 / -1; }
.row { display: flex; gap: 0.5rem; margin-top: 0.25rem; }
.actions { display: flex; gap: 0.35rem; flex-wrap: wrap; }
button.compact { padding: 0.3rem 0.55rem; font-size: 0.85rem; }
</style>
