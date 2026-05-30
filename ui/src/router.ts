import { createRouter, createWebHashHistory } from 'vue-router'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/project' },
    { path: '/project', component: { template: '<div />' } },
    { path: '/project/:projectId', redirect: to => `/project/${to.params.projectId}/reconnaissance` },
    { path: '/project/:projectId/reconnaissance', component: { template: '<div />' } },
    { path: '/project/:projectId/workspace/:workspaceId', component: { template: '<div />' } },
    { path: '/project/:projectId/workspace/:workspaceId/execution', component: { template: '<div />' } },
  ],
})

export default router
