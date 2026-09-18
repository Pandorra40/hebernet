<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'

const info = ref(null)
const error = ref('')

onMounted(async () => {
  try {
    info.value = await api('/api/server')
  } catch (e) {
    error.value = e.message
  }
})
</script>

<template>
  <div>
    <h1>Serveur</h1>
    <p class="muted">État de l’agent et stack cible</p>
    <div v-if="error" class="error-box">{{ error }}</div>
    <div v-if="info" class="card">
      <p>Agent : <strong>{{ info.agent_ok ? 'connecté' : 'hors ligne' }}</strong></p>
      <p>PHP cible : <span class="mono">{{ info.php }}</span></p>
      <p>Stack : {{ (info.stack || []).join(', ') }}</p>
      <pre class="mono muted">{{ JSON.stringify(info.agent, null, 2) }}</pre>
    </div>
  </div>
</template>

<style scoped>
h1 { margin: 0 0 0.25rem; }
pre { background: var(--bg); padding: 0.75rem; border-radius: 8px; overflow: auto; }
</style>
