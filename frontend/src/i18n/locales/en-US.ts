// English (skeleton)
//
// 第一版只实现中文；本文件是「同构骨架」，仅保留演示切换所需的少量文案。
// 未在此声明的 key 会自动回退到 zh-CN（fallbackLocale）。
// 后续补齐英文时，按 zh-CN 的结构逐段翻译即可。
export default {
  common: {
    workbench: 'Workbench',
    devConsole: 'Dev Console',
    refresh: 'Refresh',
    language: 'Language',
    theme: 'Theme (placeholder)',
    toLight: 'Switch to light',
    toDark: 'Switch to dark',
  },
  console: {
    commandNavigator: 'Command Navigator',
    expandNav: 'Expand navigator',
    collapseNav: 'Collapse navigator',
    filterPlaceholder: 'Filter commands…',
    sessionCommands: 'This session · {n} commands',
    commands: '{n} commands',
    noMatching: 'No matching commands',
    noCommands: 'No commands',
    response: 'Response',
    running: 'Running…',
    copyResponse: 'Copy response',
    copied: 'Copied',
  },
  cluster: {
    all: 'All clusters',
    switch: 'Switch cluster',
    manage: 'Manage clusters',
  },
}
