import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import ClusterHub from '../views/ClusterHub.vue'
import Workbench from '../views/Workbench.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      // 统一工作台（全屏独立页面，自带顶栏）：集群的唯一落地页，
      // 承载指标条 / 活动区 / 统一网格（索引·节点双视角）与 Dev Console 模式
      path: '/workbench/:id',
      name: 'Workbench',
      component: Workbench,
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
          // 旧版模块页（/cluster/:id/{overview,indices,shards,console}）已并入工作台，
          // 历史记录 / 书签统一收敛到对应集群的工作台
          path: 'cluster/:id/:pathMatch(.*)*',
          redirect: to => `/workbench/${String(to.params.id)}`,
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
