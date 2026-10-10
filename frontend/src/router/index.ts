import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import ClusterHub from '../views/ClusterHub.vue'
import ClusterOverview from '../views/ClusterOverview.vue'
import ClusterIndices from '../views/ClusterIndices.vue'
import ClusterShards from '../views/ClusterShards.vue'
import DevConsole from '../views/DevConsole.vue'
import WorkbenchV5 from '../views/WorkbenchV5.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      // v5 统一工作台：自带顶栏 / 指标条 / 活动区 / 统一网格的全屏独立页面
      // （与现有的 Overview / Indices / Shards 并存，互不影响）
      path: '/workbench/:id',
      name: 'WorkbenchV5',
      component: WorkbenchV5,
    },
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
