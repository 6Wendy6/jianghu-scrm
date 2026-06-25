<template>
  <Teleport to="body">
    <div v-if="open" class="modal-mask" @click.self="$emit('cancel')">
      <section class="modal">
        <h3>{{ title }}</h3>
        <p>{{ message }}</p>
        <div class="modal-actions">
          <button class="btn ghost" @click="$emit('cancel')">取消</button>
          <button class="btn primary" @click="$emit('confirm')">确认</button>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
defineProps({
  open: { type: Boolean, required: true },
  title: { type: String, required: true },
  message: { type: String, required: true }
})
defineEmits(['cancel', 'confirm'])
</script>
