<template>
  <section class="knowledge-library" aria-label="本地资料库">
    <header>
      <div><strong>本地资料库</strong><span>Markdown / PDF · 完全本机索引</span></div>
      <button type="button" :disabled="knowledge.busy" @click="knowledge.importMarkdown">
        {{ knowledge.busy ? '处理中…' : '+ 加入资料' }}
      </button>
    </header>
    <label class="mode-field">
      <span>回答方式</span>
      <select :value="knowledge.answerMode" @change="knowledge.setAnswerMode($event.target.value)">
        <option value="general">通用回答</option>
        <option value="knowledge_first">资料优先</option>
        <option value="knowledge_only">仅限资料</option>
      </select>
    </label>
    <p v-if="!knowledge.documents.length" class="empty">导入个人经历、项目说明或岗位资料，回答时将优先检索。</p>
    <ul v-else>
      <li v-for="document in knowledge.documents" :key="document.id">
        <div><strong>{{ fileName(document.path) }}</strong><span>{{ document.path }}</span></div>
        <button type="button" title="从索引删除" @click="knowledge.remove(document.path)">删除</button>
      </li>
    </ul>
    <p v-if="knowledge.error" class="error">{{ knowledge.error }}</p>
    <p v-else-if="knowledge.notice" class="notice">{{ knowledge.notice }}</p>
  </section>
</template>

<script setup>
import { onMounted } from 'vue'
import { useKnowledgeStore } from '../stores/knowledge'
const knowledge = useKnowledgeStore()
const fileName = (path) => String(path || '').split(/[\\/]/).pop()
onMounted(knowledge.refresh)
</script>

<style scoped>
.knowledge-library { min-height: 0; min-width: 0; overflow: auto; padding: var(--sp-4); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface-card); pointer-events: auto; }
header, li { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-3); }
header div, li div { min-width: 0; }
header strong, header span, li strong, li span { display: block; }
header strong { color: var(--text-primary); font-size: var(--text-sm); }
header span, li span { overflow: hidden; color: var(--text-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
button { border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-card); color: var(--accent); cursor: pointer; font-size: var(--text-xs); padding: 6px 9px; }
.mode-field { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-2); margin-top: var(--sp-3); color: var(--text-secondary); font-size: var(--text-xs); }
.mode-field select { min-width: 108px; padding: 5px 7px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-primary); }
ul { display: grid; gap: var(--sp-2); margin: var(--sp-3) 0 0; padding: 0; list-style: none; }
li { padding-top: var(--sp-2); border-top: 1px solid var(--border-subtle); }
li strong { color: var(--text-primary); font-size: var(--text-xs); }
.empty { color: var(--text-secondary); font-size: var(--text-xs); line-height: 1.5; }
.error { color: var(--color-error); font-size: var(--text-xs); }
.notice { color: var(--color-success); font-size: var(--text-xs); }
</style>
