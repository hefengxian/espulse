import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'virtual:uno.css'
// 变量字体：一个文件覆盖 100-900，不再出现「只加载 400 却用 600/700」的伪造粗体
import '@fontsource-variable/inter/wght.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import './style.less'
import App from './App.vue'
import router from './router'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.mount('#app')
