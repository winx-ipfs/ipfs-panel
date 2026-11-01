import type { MessageSchema } from './en'

export const zh: MessageSchema = {
  brand: 'ipfs-panel',
  tagline: 'Kubo 面板',
  languageLabel: '语言',
  languageEn: 'EN',
  languageZh: '中文',
  navLabel: '栏目',
  navEmpty: '还没有栏目。',
  emptyTitle: '空壳',
  emptyBody: '现在只有布局和语言。后面的切片才会加上登录、启停、文件、固定（pin）和场景配置。',
  emptyNote: '这个页面不会去叫 Kubo。管理服务是另一个进程，Kubo RPC 也不会暴露到公网。',
}
