import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import Dashboard from '../views/Dashboard.vue'
import DevConsole from '../views/DevConsole.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      component: MainLayout,
      children: [
        {
          path: '',
          name: 'Dashboard',
          component: Dashboard,
        },
        {
          path: 'console',
          name: 'DevConsole',
          component: DevConsole,
        },
        // Other routes can be added here
        {
          path: ':pathMatch(.*)*',
          redirect: '/',
        },
      ],
    },
  ],
})

export default router
