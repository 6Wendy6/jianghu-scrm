<template>
  <div class="tabs">
    <button v-for="tab in tabs" :key="tab.key" :class="{ active: tab.key === modelValue }" @click="$emit('update:modelValue', tab.key)">
      {{ tab.label }}
    </button>
  </div>
</template>

<script setup>
defineProps({
  modelValue: { type: String, required: true },
  tabs: { type: Array, required: true }
})
defineEmits(['update:modelValue'])
</script>
