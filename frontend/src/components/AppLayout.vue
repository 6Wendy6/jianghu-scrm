<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span>私</span><strong>美梦 SCRM</strong></div>
      <button v-for="item in nav" :key="item.key" class="nav-item" :class="{ active: item.key === modelValue }" @click="$emit('update:modelValue', item.key)">
        {{ item.label }}
      </button>
    </aside>
    <main class="main">
      <header class="topbar">
        <h1>{{ title }}</h1>
        <p>线下门店私域客户池 MVP · 门店引流、归因锁定、身份合并、触达、交接</p>
      </header>
      <section class="content"><slot /></section>
    </main>
  </div>
</template>

<script setup>
defineProps({
  modelValue: { type: String, required: true },
  nav: { type: Array, required: true },
  title: { type: String, required: true }
})
defineEmits(['update:modelValue'])
</script>
