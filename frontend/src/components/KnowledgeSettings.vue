<template>
  <div class="knowledge-settings">
    <section class="hero">
      <div class="hero-icon"><Icon name="file" :size="28" /></div>
      <div>
        <h2>本地知识资料库</h2>
        <p>Markdown 与带文本层的 PDF 会在本机建立全文索引，面试回答可直接引用。</p>
      </div>
      <div class="hero-actions">
        <button class="primary-action" :disabled="knowledge.busy" @click="knowledge.importMarkdown">{{ knowledge.busy ? '处理中…' : '选择文件' }}</button>
        <button class="secondary-action" :disabled="knowledge.busy" @click="knowledge.importDirectory">选择目录</button>
        <button class="secondary-action" :disabled="knowledge.busy" @click="showEditor = !showEditor">新建资料</button>
      </div>
    </section>

    <section v-if="showEditor" class="note-editor">
      <div class="section-title"><strong>新建本地资料</strong><span>保存为 Markdown 并立即建立索引</span></div>
      <input v-model.trim="noteTitle" placeholder="资料标题，例如：我的项目经历" />
      <textarea v-model="noteContent" placeholder="粘贴或输入资料内容"></textarea>
      <div class="editor-actions">
        <button class="secondary-action" @click="showEditor = false">取消</button>
        <button class="primary-action" :disabled="knowledge.busy || !noteTitle || !noteContent.trim()" @click="saveNote">保存并索引</button>
      </div>
    </section>

    <section class="control-card">
      <label>
        <span>回答策略</span>
        <select :value="knowledge.answerMode" @change="knowledge.setAnswerMode($event.target.value)">
          <option value="general">通用回答</option>
          <option value="knowledge_first">资料优先</option>
          <option value="knowledge_only">仅资料库</option>
        </select>
      </label>
      <button class="secondary-action" :disabled="knowledge.busy || !knowledge.documents.length" @click="knowledge.reindex">
        <Icon name="refresh" :size="15" />重建索引
      </button>
    </section>

    <section class="search-card">
      <div class="search-box">
        <Icon name="search" :size="16" />
        <input v-model="knowledge.query" placeholder="输入关键词测试本地检索" @keyup.enter="knowledge.search" />
        <button @click="knowledge.search">检索</button>
      </div>
      <ul v-if="knowledge.results.length" class="results">
        <li v-for="item in knowledge.results.slice(0, 5)" :key="item.chunk?.id || item.path">
          <strong>{{ fileName(item.chunk?.path || item.path) }}</strong>
          <span>{{ item.chunk?.content || item.content }}</span>
        </li>
      </ul>
    </section>

    <section class="files">
      <div class="section-title">
        <div><strong>已加入文件</strong><span>{{ knowledge.documents.length }} 个</span></div>
        <span class="format">支持 .md、.markdown、.pdf</span>
      </div>
      <p class="storage-path" :title="knowledge.storagePath">本机保存目录：{{ knowledge.storagePath }}</p>
      <div v-if="!knowledge.documents.length" class="empty">
        <Icon name="file" :size="26" />
        <strong>资料库还是空的</strong>
        <span>点击“加入资料”，可一次选择多个文件。</span>
      </div>
      <ul v-else class="file-list">
        <li v-for="document in knowledge.documents" :key="document.id">
          <div class="file-icon">{{ extension(document.path) }}</div>
          <div class="file-meta"><strong>{{ fileName(document.path) }}</strong><span :title="document.path">{{ document.path }}</span></div>
          <button class="icon-action" title="删除" @click="knowledge.remove(document.path)"><Icon name="trash" :size="16" /></button>
        </li>
      </ul>
    </section>

    <p v-if="knowledge.error" class="message error">{{ knowledge.error }}</p>
    <p v-else-if="knowledge.notice" class="message success">{{ knowledge.notice }}</p>
    <p class="footnote">扫描版 PDF 需要先带有 OCR 文本层；索引内容与数据库均保存在当前电脑。</p>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { useKnowledgeStore } from '../stores/knowledge'

const knowledge = useKnowledgeStore()
const showEditor = ref(false)
const noteTitle = ref('')
const noteContent = ref('')
const fileName = path => String(path || '').split(/[\\/]/).pop()
const extension = path => String(path || '').split('.').pop().toUpperCase()
async function saveNote() {
  if (await knowledge.saveNote(noteTitle.value, noteContent.value)) {
    noteTitle.value = ''
    noteContent.value = ''
    showEditor.value = false
  }
}
onMounted(knowledge.refresh)
</script>

<style scoped>
.knowledge-settings { display: grid; gap: 14px; }
.hero, .control-card, .search-card, .files { border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface-card); }
.hero { display: grid; grid-template-columns: auto 1fr; align-items: center; gap: 14px; padding: 18px; }
.hero-actions { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 8px; }
.hero-icon { width: 48px; height: 48px; display: grid; place-items: center; color: var(--accent); background: var(--accent-muted); border-radius: 14px; }
h2 { margin: 0; color: var(--text-primary); font-size: var(--text-lg); } .hero p { margin: 4px 0 0; color: var(--text-secondary); font-size: var(--text-xs); line-height: 1.5; }
button, select, input { font: inherit; }
.primary-action, .secondary-action, .search-box button { border-radius: var(--radius-sm); cursor: pointer; font-weight: var(--weight-semibold); }
.primary-action { padding: 10px 14px; border: 0; color: var(--text-inverse); background: var(--accent); }
.note-editor { display: grid; gap: 10px; padding: 14px; border: 1px solid var(--accent-border); border-radius: var(--radius-md); background: var(--accent-muted); }
.note-editor input, .note-editor textarea { width: 100%; box-sizing: border-box; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-primary); font: inherit; }
.note-editor input { padding: 9px 11px; }.note-editor textarea { min-height: 120px; padding: 11px; resize: vertical; line-height: 1.6; }.editor-actions { display: flex; justify-content: flex-end; gap: 8px; }
.control-card { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; }
.control-card label { display: flex; align-items: center; gap: 10px; color: var(--text-secondary); font-size: var(--text-xs); }
select { padding: 7px 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-primary); }
.secondary-action { display: inline-flex; align-items: center; gap: 6px; padding: 7px 10px; border: 1px solid var(--border-default); background: var(--surface-elevated); color: var(--text-primary); }
.search-card { padding: 12px; }.search-box { display: flex; align-items: center; gap: 8px; padding: 7px 8px 7px 11px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-muted); }
.search-box input { flex: 1; min-width: 0; border: 0; outline: 0; background: transparent; color: var(--text-primary); }.search-box button { padding: 6px 11px; border: 0; color: var(--accent); background: var(--accent-muted); }
.results, .file-list { margin: 10px 0 0; padding: 0; list-style: none; }.results li { display: grid; gap: 3px; padding: 8px 4px; border-top: 1px solid var(--border-subtle); }.results strong { color: var(--text-primary); font-size: var(--text-xs); }.results span { max-height: 36px; overflow: hidden; color: var(--text-secondary); font-size: 11px; line-height: 1.5; }
.files { padding: 14px; }.section-title { display: flex; justify-content: space-between; gap: 10px; }.section-title div { display: flex; gap: 8px; }.section-title strong { color: var(--text-primary); font-size: var(--text-sm); }.section-title span, .format { color: var(--text-muted); font-size: var(--text-xs); }
.storage-path { overflow: hidden; margin: 8px 0 0; color: var(--text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.empty { min-height: 100px; display: grid; place-items: center; align-content: center; gap: 5px; color: var(--text-muted); }.empty strong { color: var(--text-secondary); font-size: var(--text-sm); }.empty span { font-size: var(--text-xs); }
.file-list li { display: flex; align-items: center; gap: 10px; padding: 9px 0; border-top: 1px solid var(--border-subtle); }.file-icon { width: 34px; height: 34px; display: grid; place-items: center; flex: 0 0 auto; border-radius: 9px; color: var(--accent); background: var(--accent-muted); font-size: 10px; font-weight: 700; }.file-meta { flex: 1; min-width: 0; }.file-meta strong, .file-meta span { display: block; }.file-meta strong { color: var(--text-primary); font-size: var(--text-xs); }.file-meta span { overflow: hidden; color: var(--text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.icon-action { width: 30px; height: 30px; display: grid; place-items: center; border: 0; border-radius: 8px; color: var(--text-muted); background: transparent; cursor: pointer; }.icon-action:hover { color: var(--color-error); background: var(--error-bg); }
.message { margin: 0; padding: 8px 10px; border-radius: var(--radius-sm); font-size: var(--text-xs); }.error { color: var(--color-error); background: var(--error-bg); }.success { color: var(--color-success); background: var(--success-bg); }.footnote { margin: 0; color: var(--text-muted); font-size: 10px; }
button:disabled { opacity: .45; cursor: not-allowed; }
@media (max-width: 560px) { .hero { grid-template-columns: auto 1fr; }.primary-action { grid-column: 1 / -1; }.control-card { align-items: stretch; flex-direction: column; } }
</style>
