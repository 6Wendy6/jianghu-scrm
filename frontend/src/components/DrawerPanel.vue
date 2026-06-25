<template>
  <Teleport to="body">
    <div v-if="open" class="drawer-mask" @click.self="$emit('close')">
      <aside class="drawer">
        <header>
          <h2>{{ title }}</h2>
          <button class="btn ghost" @click="$emit('close')">关闭</button>
        </header>
        <slot />
      </aside>
    </div>
  </Teleport>
</template>

<script setup>
defineProps({
  open: { type: Boolean, required: true },
  title: { type: String, default: '' }
})
defineEmits(['close'])
</script>
