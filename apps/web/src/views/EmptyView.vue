<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <section class="empty">
    <h1>{{ t('emptyTitle') }}</h1>
    <p>{{ t('emptyBody') }}</p>
    <p class="note">{{ t('emptyNote') }}</p>
  </section>
</template>
