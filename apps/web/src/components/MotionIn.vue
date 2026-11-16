<script setup lang="ts">
import { motion } from 'motion-v'
import { useMotion } from '@/lib/motion'

const { enter, shown, transition } = useMotion()
</script>

<template>
  <motion.div :initial="enter" :animate="shown" :transition="transition">
    <slot />
  </motion.div>
</template>
