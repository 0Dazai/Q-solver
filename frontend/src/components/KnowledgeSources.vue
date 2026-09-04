<template>
  <details v-if="items.length" class="knowledge-sources">
    <summary class="sources-summary">
      <span class="sources-title">引用资料 <span class="sources-count">{{ items.length }} 条</span></span>
      <span class="toggle-label"><span class="expand-label">展开</span><span class="collapse-label">收起</span><span class="chevron" aria-hidden="true"></span></span>
    </summary>
    <div class="sources-content">
      <article v-for="item in items" :key="item.id" class="source-item">
        <strong>{{ fileName(item.path) }}</strong>
        <span>{{ item.titlePath || '未命名段落' }}</span>
        <p>{{ item.content }}</p>
      </article>
    </div>
  </details>
</template>

<script setup>
defineProps({ items: { type: Array, default: () => [] } })
const fileName = (path) => String(path || '').split(/[\\/]/).pop()
</script>

<style scoped>
.knowledge-sources { margin-top: var(--sp-3); border-radius: var(--radius-sm); background: var(--surface-input); border: 1px solid var(--border-subtle); overflow: hidden; }
.sources-summary { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-3); padding: var(--sp-3); cursor: pointer; list-style: none; user-select: none; }
.sources-summary::-webkit-details-marker { display: none; }
.sources-summary:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
.sources-title { color: var(--text-primary); font-size: var(--text-sm); font-weight: var(--weight-bold); }
.sources-count { margin-left: 4px; color: var(--text-muted); font-size: var(--text-xs); font-weight: var(--weight-medium); }
.toggle-label { display: inline-flex; align-items: center; gap: 6px; color: var(--text-muted); font-size: var(--text-xs); }
.collapse-label { display: none; }
.chevron { width: 7px; height: 7px; border-right: 1.5px solid currentColor; border-bottom: 1.5px solid currentColor; transform: rotate(45deg) translateY(-2px); transition: transform 160ms ease; }
.knowledge-sources[open] .expand-label { display: none; }
.knowledge-sources[open] .collapse-label { display: inline; }
.knowledge-sources[open] .chevron { transform: rotate(225deg) translate(-1px, -1px); }
.sources-content { padding: 0 var(--sp-3) var(--sp-3); }
.source-item { padding-top: var(--sp-2); border-top: 1px solid var(--border-subtle); }
.source-item strong, .source-item span { display: block; font-size: var(--text-xs); }
.source-item span { margin-top: 2px; color: var(--text-muted); }
.source-item p { display: -webkit-box; overflow: hidden; margin: 5px 0 0; color: var(--text-secondary); font-size: var(--text-xs); line-height: 1.5; -webkit-line-clamp: 3; -webkit-box-orient: vertical; }
</style>
