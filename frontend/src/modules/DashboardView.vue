<template>
  <div>
    <PageHeader title="闭环总览" description="看本期门店私域闭环是否健康，重点盯入池、归因、待办、触达、交接。" />
    <div class="kpi-grid">
      <KpiCard label="今日入池客户" :value="summary.todayPool || 0" hint="扫码到加企微完成率 68%" />
      <KpiCard label="归因完整率" :value="summary.attributionRate || '0%'" hint="来源门店/品牌/首次导购齐全" />
      <KpiCard label="待处理事项" :value="summary.pending || 0" hint="交接、待办、缺标签" />
      <KpiCard label="触达覆盖率" :value="summary.touchRate || '0%'" hint="欢迎语/待办/群运营至少一次" />
    </div>
    <div class="grid two">
      <section class="panel">
        <h3>门店入池趋势</h3>
        <div class="bars">
          <span v-for="(item, index) in summary.trend || []" :key="index" :style="{ height: `${item}%` }">{{ index + 1 }}</span>
        </div>
      </section>
      <section class="panel">
        <h3>AI 经营建议</h3>
        <div class="list-row" v-for="item in summary.suggestions || []" :key="item">{{ item }}<button class="btn ghost">查看</button></div>
      </section>
    </div>
  </div>
</template>

<script setup>
import PageHeader from '../components/PageHeader.vue'
import KpiCard from '../components/KpiCard.vue'
defineProps({ summary: { type: Object, required: true } })
</script>
