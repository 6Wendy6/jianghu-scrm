<template>
  <div class="filter-bar">
    <slot />
    <button class="btn" @click="$emit('search')">查询</button>
    <button class="btn ghost" @click="$emit('reset')">重置</button>
  </div>
</template>

<script setup>
defineEmits(['search', 'reset'])
</script>
