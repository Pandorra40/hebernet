import { createRouter, createWebHistory } from 'vue-router'
import { getToken, getUser } from './api'
import LoginView from './views/LoginView.vue'
import AdminLayout from './layouts/AdminLayout.vue'
import ClientLayout from './layouts/ClientLayout.vue'
import AdminSites from './views/admin/AdminSites.vue'
import AdminSiteDetail from './views/admin/AdminSiteDetail.vue'
import AdminPackages from './views/admin/AdminPackages.vue'
import AdminUsers from './views/admin/AdminUsers.vue'
import AdminServer from './views/admin/AdminServer.vue'
import ClientSites from './views/client/ClientSites.vue'
import ClientSiteDetail from './views/client/ClientSiteDetail.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/admin',
      component: AdminLayout,
      meta: { role: 'admin' },
      children: [
        { path: '', redirect: '/admin/sites' },
        { path: 'sites', component: AdminSites },
        { path: 'sites/:id', component: AdminSiteDetail, props: true },
        { path: 'packages', component: AdminPackages },
        { path: 'users', component: AdminUsers },
        { path: 'server', component: AdminServer },
      ],
    },
    {
      path: '/client',
      component: ClientLayout,
      meta: { role: 'client' },
      children: [
        { path: '', redirect: '/client/sites' },
        { path: 'sites', component: ClientSites },
        { path: 'sites/:id', component: ClientSiteDetail, props: true },
      ],
    },
    { path: '/', redirect: '/login' },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  const token = getToken()
  const user = getUser()
  if (!token || !user) return { path: '/login' }
  if (to.meta.role && user.role !== to.meta.role) {
    return user.role === 'admin' ? '/admin/sites' : '/client/sites'
  }
  return true
})

export default router
