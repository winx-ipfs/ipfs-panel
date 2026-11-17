<script setup lang="ts">
import { MoreHorizontal } from '@lucide/vue'
import {
  DropdownMenuContent,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import IconButton from '@/components/IconButton.vue'

defineProps<{
  label: string
  menuLabel: string
  menuClass: string
}>()
</script>

<template>
  <!--
    The list pages keep this menu in a table cell, and those cells clip their
    overflow so long names and CIDs get an ellipsis. An absolutely positioned
    menu inside the cell was therefore invisible or unusable past the first
    row. The primitive portals the content to the body and flips it when the
    viewport runs out, which is the part that cannot be done by hand cheaply.
  -->
  <DropdownMenuRoot>
    <DropdownMenuTrigger as-child>
      <IconButton :label="label">
        <MoreHorizontal :size="18" :stroke-width="1.75" aria-hidden="true" />
      </IconButton>
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        :class="['row-menu', menuClass]"
        :aria-label="menuLabel"
        align="end"
        :side-offset="4"
        :collision-padding="8"
        :loop="true"
      >
        <slot />
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>