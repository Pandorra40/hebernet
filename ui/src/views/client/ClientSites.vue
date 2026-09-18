<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { api } from '../../api'

const router = useRouter()
const sites = ref([])
const error = ref('')

async function load() {
  sites.value = await api('/api/sites')
}

function manage(id) {
  router.push(`/client/sites/${id}`)
}

onMounted(async () => {
  try {
    await load()
  } catch (e) {
    error.value = e.message
  }
})
</script>

<template>
  <div>
    <header class="head">
      <div>
        <h1>Mes sites</h1>
        <p class="muted">Gérez SFTP, SSL, base de données, quota et logs</p>
      </div>
    </header>
    <div v-if="error" class="error-box">{{ error }}</div>

    <div v-if="!sites.length && !error" class="card empty">
      <h2>Aucun site pour le moment</h2>
      <p class="muted">Quand l’hébergeur vous assignera un site, il apparaîtra ici avec un bouton <strong>Gérer</strong>.</p>
    </div>

    <div v-else class="list">
      <article v-for="s in sites" :key="s.id" class="card site">
        <div>
          <h2><RouterLink :to="`/client/sites/${s.id}`">{{ s.domain }}</RouterLink></h2>
          <p class="muted">
            {{ s.app_type }} · PHP {{ s.php_version }}
            · <span class="badge" :class="s.status">{{ s.status }}</span>
            · {{ s.quota_used_mb }} / {{ s.quota_mb }} Mo
          </p>
        </div>
        <button type="button" @click="manage(s.id)">Gérer</button>
      </article>
    </div>
  </div>
</template>

<style scoped>
.head { margin-bottom: 1.25rem; }
h1 { margin: 0 0 0.25rem; }
.list { display: flex; flex-direction: column; gap: 0.75rem; }
.site {
  display: flex; justify-content: space-between; align-items: center; gap: 1rem; flex-wrap: wrap;
}
.site h2 { margin: 0 0 0.35rem; font-size: 1.15rem; }
.empty h2 { margin: 0 0 0.5rem; font-size: 1.1rem; }
</style>
