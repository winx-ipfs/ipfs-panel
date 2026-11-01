import type { MessageSchema } from './en'

declare module 'vue-i18n' {
  export interface DefineLocaleMessage extends MessageSchema {}
}
