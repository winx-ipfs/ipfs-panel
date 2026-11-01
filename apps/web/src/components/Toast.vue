<script setup lang="ts">
import { useToastStore } from '@/stores/toast'
const toast = useToastStore()
</script>

<template>
  <div class="ui-toaster" aria-live="polite" aria-relevant="additions">
    <p v-for="item in toast.items" :key="item.id" class="ui-toast" role="status">{{ item.message }}</p>
  </div>
</template>
