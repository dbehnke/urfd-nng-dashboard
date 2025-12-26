import { createRouter, createWebHistory } from 'vue-router'
import LastHeard from '../views/LastHeard.vue'

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes: [
        {
            path: '/',
            name: 'last-heard',
            component: LastHeard
        },
        {
            path: '/nodes',
            name: 'nodes',
            component: () => import('../views/Nodes.vue')
        },
        {
            path: '/users',
            name: 'users',
            component: () => import('../views/Users.vue')
        },
        {
            path: '/peers',
            name: 'peers',
            component: () => import('../views/Peers.vue')
        },
        {
            path: '/modules',
            name: 'modules',
            component: () => import('../views/Modules.vue')
        }
    ]
})

export default router
