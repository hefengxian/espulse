import {
  defineConfig,
  presetAttributify,
  presetIcons,
  presetTypography,
  presetWind3,
  transformerDirectives,
  transformerVariantGroup,
} from 'unocss'

export default defineConfig({
  shortcuts: [
    ['flex-center', 'flex items-center justify-center'],
    ['btn-icon', 'w-8 h-8 rounded-7px border border-border bg-transparent text-text-2 flex-center cursor-pointer transition-all hover:bg-bg-3 hover:text-text hover:border-border-2'],
    // 分片分布矩阵里的分片方块
    ['chip', 'w-4 h-4 rounded-3px text-10px font-mono flex-center leading-none flex-shrink-0'],
  ],
  theme: {
    colors: {
      bg: 'var(--esp-bg)',
      'bg-2': 'var(--esp-bg-2)',
      'bg-3': 'var(--esp-bg-3)',
      'bg-4': 'var(--esp-bg-4)',
      border: 'var(--esp-border)',
      'border-2': 'var(--esp-border-2)',
      text: 'var(--esp-text)',
      'text-2': 'var(--esp-text-2)',
      'text-3': 'var(--esp-text-3)',
      accent: 'var(--esp-accent)',
      'accent-glow': 'var(--esp-accent-glow)',
      green: 'var(--esp-green)',
      'green-bg': 'var(--esp-green-bg)',
      'green-border': 'var(--esp-green-border)',
      yellow: 'var(--esp-yellow)',
      'yellow-bg': 'var(--esp-yellow-bg)',
      'yellow-border': 'var(--esp-yellow-border)',
      red: 'var(--esp-red)',
      'red-bg': 'var(--esp-red-bg)',
      'red-border': 'var(--esp-red-border)',
    },
    fontFamily: {
      sans: 'var(--esp-font-sans)',
      mono: 'var(--esp-font-mono)',
    },
  },
  presets: [
    presetWind3(),
    presetAttributify(),
    presetIcons({
      warn: true,
      // 显式注册图标集：文档推荐的确定性写法。
      // 依赖 Node 端"自动探测已安装图标集"在 Vite 配置加载环境下不可靠。
      collections: {
        lucide: () => import('@iconify-json/lucide/icons.json').then(i => i.default as any),
      },
    }),
    presetTypography(),
  ],
  transformers: [
    transformerDirectives(),
    transformerVariantGroup(),
  ],
  preflights: [
    {
      getCSS: () => `
        button, input, select, textarea {
          background-color: transparent;
          border-color: inherit;
          border-style: solid;
          border-width: 0;
          color: inherit;
          font-family: inherit;
          font-size: inherit;
          font-weight: inherit;
          line-height: inherit;
          margin: 0;
          padding: 0;
        }
        button {
          cursor: pointer;
        }
      `,
    },
  ],
})
