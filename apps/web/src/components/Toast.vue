<script setup lang="ts">
import { motion } from 'motion-v'
import { useMotion } from '@/lib/motion'
import { useToastStore } from '@/stores/toast'

const toast = useToastStore()
const { enter, shown, transition } = useMotion()
</script>

<template>
  <div class="ui-toaster">
    <motion.p
      v-for="item in toast.items"
      :key="item.id"
      class="ui-toast"
      role="status"
      :initial="enter"
      :animate="shown"
      :transition="transition"
    >
      {{ item.message }}
    </motion.p>
  </div>
</template>
