import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import ClusterHub from '../views/ClusterHub.vue'
import ClusterOverview from '../views/ClusterOverview.vue'
import ClusterIndices from '../views/ClusterIndices.vue'
import ClusterShards from '../views/ClusterShards.vue'
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
          name: 'ClusterHub',
          component: ClusterHub,
        },
        {
          path: 'cluster/:id/overview',
          name: 'ClusterOverview',
          component: ClusterOverview,
        },
        {
          path: 'cluster/:id/indices',
          name: 'ClusterIndices',
          component: ClusterIndices,
        },
        {
          path: 'cluster/:id/shards',
          name: 'ClusterShards',
          component: ClusterShards,
        },
        {
          path: 'cluster/:id/console',
          name: 'DevConsole',
          component: DevConsole,
        },
        {
          path: ':pathMatch(.*)*',
          redirect: '/',
        },
      ],
    },
  ],
})

export default router
