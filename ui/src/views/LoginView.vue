<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, setSession } from '../api'

const router = useRouter()
const email = ref('admin@hebernet.local')
const password = ref('admin')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const data = await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: email.value, password: password.value }),
    })
    setSession(data.token, data.user)
    router.push(data.user.role === 'admin' ? '/admin/sites' : '/client/sites')
  } catch (e) {
    error.value = e.message || 'Échec de connexion'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <form class="card panel" @submit.prevent="submit">
      <div class="brand">
        <span class="mark">H</span>
        <div>
          <h1>Hébernet</h1>
          <p class="muted">Panneau d’hébergement</p>
        </div>
      </div>
      <div v-if="error" class="error-box">{{ error }}</div>
      <div class="field">
        <label>E-mail</label>
        <input v-model="email" type="email" autocomplete="username" required />
      </div>
      <div class="field">
        <label>Mot de passe</label>
        <input v-model="password" type="password" autocomplete="current-password" required />
      </div>
      <button type="submit" :disabled="loading">
        {{ loading ? 'Connexion…' : 'Se connecter' }}
      </button>
      <p class="hint muted">
        Démo : <span class="mono">admin@hebernet.local / admin</span>
        · <span class="mono">client@hebernet.local / client</span>
      </p>
    </form>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 1.5rem;
  background:
    radial-gradient(ellipse at 20% 10%, #1a3a34 0%, transparent 50%),
    radial-gradient(ellipse at 80% 90%, #1a2430 0%, transparent 45%),
    var(--bg);
}
.panel { width: min(420px, 100%); }
.brand { display: flex; gap: 0.9rem; align-items: center; margin-bottom: 1.5rem; }
.brand h1 { margin: 0; font-size: 1.4rem; }
.brand p { margin: 0.15rem 0 0; }
.mark {
  width: 44px; height: 44px; border-radius: 12px;
  background: linear-gradient(145deg, #3d9a8b, #1f5c52);
  display: grid; place-items: center; font-weight: 700; font-size: 1.2rem; color: #e8fff9;
}
button { width: 100%; margin-top: 0.25rem; }
.hint { margin-top: 1.25rem; font-size: 0.82rem; line-height: 1.5; }
</style>
