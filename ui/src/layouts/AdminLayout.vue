<script setup>
import { computed } from 'vue'
import { useRouter, useRoute, RouterLink, RouterView } from 'vue-router'
import { clearSession, getUser } from '../api'

const router = useRouter()
const route = useRoute()
const user = computed(() => getUser())

const links = [
  { to: '/admin/sites', label: 'Sites' },
  { to: '/admin/packages', label: 'Packages' },
  { to: '/admin/users', label: 'Utilisateurs' },
  { to: '/admin/server', label: 'Serveur' },
]

function logout() {
  clearSession()
  router.push('/login')
}

function active(path) {
  return route.path.startsWith(path)
}
</script>

<template>
  <div class="shell">
    <aside class="side">
      <div class="brand">
        <span class="mark">H</span>
        <div>
          <strong>Hébernet</strong>
          <div class="role">Administration</div>
        </div>
      </div>
      <nav>
        <RouterLink
          v-for="l in links"
          :key="l.to"
          :to="l.to"
          :class="{ on: active(l.to) }"
        >{{ l.label }}</RouterLink>
      </nav>
      <div class="foot">
        <div class="muted small">{{ user?.email }}</div>
        <button class="secondary" type="button" @click="logout">Déconnexion</button>
      </div>
    </aside>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.shell { display: grid; grid-template-columns: 240px 1fr; min-height: 100vh; }
.side {
  background: var(--bg-elevated);
  border-right: 1px solid var(--border);
  padding: 1.25rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
.brand { display: flex; gap: 0.75rem; align-items: center; }
.mark {
  width: 36px; height: 36px; border-radius: 9px;
  background: linear-gradient(145deg, #3d9a8b, #1f5c52);
  display: grid; place-items: center; font-weight: 700; color: #e8fff9;
}
.role { font-size: 0.8rem; color: var(--muted); }
nav { display: flex; flex-direction: column; gap: 0.25rem; flex: 1; }
nav a {
  color: var(--muted);
  padding: 0.55rem 0.75rem;
  border-radius: 8px;
  text-decoration: none;
}
nav a.on, nav a:hover { background: var(--bg-soft); color: var(--text); text-decoration: none; }
.foot { display: flex; flex-direction: column; gap: 0.5rem; }
.small { font-size: 0.8rem; }
.main { padding: 1.75rem 2rem; max-width: 1100px; }
@media (max-width: 800px) {
  .shell { grid-template-columns: 1fr; }
  .side { border-right: none; border-bottom: 1px solid var(--border); }
}
</style>
