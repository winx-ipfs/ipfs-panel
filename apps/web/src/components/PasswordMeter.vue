<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { passwordLevel, type PasswordLevel } from '@/auth/strength'

const props = defineProps<{ password: string }>()
const { t } = useI18n()

const level = computed(() => passwordLevel(props.password))

const bars = computed(() => {
  const scores: Record<PasswordLevel, number> = { empty: 0, short: 1, weak: 2, ok: 3, strong: 4 }
  return scores[level.value]
})

const label = computed(() => {
  if (level.value === 'short') return t('strengthShort')
  if (level.value === 'weak') return t('strengthWeak')
  if (level.value === 'ok') return t('strengthOk')
  if (level.value === 'strong') return t('strengthStrong')
  return t('setupPasswordHint')
})
</script>

<template>
  <div class="meter-block">
    <div
      class="meter"
      :class="`is-${level}`"
      role="meter"
      :aria-valuemin="0"
      :aria-valuemax="4"
      :aria-valuenow="bars"
      :aria-valuetext="label"
      :aria-label="t('passwordStrength')"
    >
      <span v-for="index in 4" :key="index" :class="{ on: index <= bars }" />
    </div>
    <p class="meter-label">{{ label }}</p>
  </div>
</template>
