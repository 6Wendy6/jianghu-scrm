<template>
  <span class="status-tag" :class="tone">{{ text }}</span>
</template>

<script setup>
defineProps({
  text: { type: String, required: true },
  tone: { type: String, default: 'blue' }
})
</script>
