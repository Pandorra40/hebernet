<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'

const users = ref([])
const error = ref('')
const form = ref({ email: '', password: '', role: 'client', display_name: '' })

async function load() {
  users.value = await api('/api/users')
}

async function createUser() {
  error.value = ''
  try {
    await api('/api/users', { method: 'POST', body: JSON.stringify(form.value) })
    form.value = { email: '', password: '', role: 'client', display_name: '' }
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
      <h1>Utilisateurs</h1>
      <p class="muted">Comptes admin et clients</p>
    </header>
    <div v-if="error" class="error-box">{{ error }}</div>

    <div class="card form">
      <div class="grid">
        <div class="field"><label>E-mail</label><input v-model="form.email" type="email" /></div>
        <div class="field"><label>Mot de passe</label><input v-model="form.password" type="password" /></div>
        <div class="field"><label>Nom</label><input v-model="form.display_name" /></div>
        <div class="field">
          <label>Rôle</label>
          <select v-model="form.role">
            <option value="client">Client</option>
            <option value="admin">Admin</option>
          </select>
        </div>
      </div>
      <button type="button" :disabled="!form.email || !form.password" @click="createUser">Créer</button>
    </div>

    <div class="card">
      <table>
        <thead><tr><th>E-mail</th><th>Nom</th><th>Rôle</th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.email }}</td>
            <td>{{ u.display_name }}</td>
            <td><span class="badge">{{ u.role }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.head { margin-bottom: 1.25rem; }
h1 { margin: 0 0 0.25rem; }
.form { margin-bottom: 1rem; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
</style>
