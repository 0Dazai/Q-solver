import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '../services/api'

export const useKnowledgeStore = defineStore('knowledge', () => {
  const documents = ref([])
  const busy = ref(false)
  const error = ref('')
  const notice = ref('')
  const answerMode = ref('knowledge_first')
  const query = ref('')
  const results = ref([])
  const storagePath = ref('')

  async function refresh() {
    try {
      const [items, settings, path] = await Promise.all([api.listKnowledgeDocuments(), api.getSettings(), api.getKnowledgeStoragePath()])
      documents.value = Array.isArray(items) ? items : []
      storagePath.value = path
      answerMode.value = settings?.knowledge?.answerMode || 'knowledge_first'
    } catch (reason) {
      error.value = reason?.message || String(reason)
    }
  }

  async function importDirectory() {
    busy.value = true
    error.value = ''
    notice.value = ''
    try {
      const path = await api.selectKnowledgeDirectory()
      if (!path) return
      await api.indexKnowledgeDirectory(path)
      await refresh()
      notice.value = '目录资料已加入并建立索引'
    } catch (reason) {
      error.value = reason?.message || String(reason)
    } finally {
      busy.value = false
    }
  }

  async function saveNote(title, content) {
    busy.value = true
    error.value = ''
    notice.value = ''
    try {
      const path = await api.saveKnowledgeNote(title, content)
      await refresh()
      notice.value = `资料已保存：${path}`
      return true
    } catch (reason) {
      error.value = reason?.message || String(reason)
      return false
    } finally {
      busy.value = false
    }
  }

  async function importMarkdown() {
    busy.value = true
    error.value = ''
    notice.value = ''
    try {
      const paths = await api.selectKnowledgeFiles()
      if (!paths?.length) return
      await api.indexKnowledgeFiles(paths)
      await refresh()
      notice.value = `已加入 ${paths.length} 个资料文件`
    } catch (reason) {
      error.value = reason?.message || String(reason)
    } finally {
      busy.value = false
    }
  }

  async function remove(path) {
    busy.value = true
    error.value = ''
    notice.value = ''
    try {
      await api.deleteKnowledgeDocument(path)
      await refresh()
      notice.value = '文件已从资料库移除'
    } catch (reason) {
      error.value = reason?.message || String(reason)
    } finally {
      busy.value = false
    }
  }

  async function setAnswerMode(mode) {
    const previous = answerMode.value
    answerMode.value = mode
    const message = await api.setKnowledgeAnswerMode(mode)
    if (message) {
      answerMode.value = previous
      error.value = message
    }
  }

  async function reindex() {
    busy.value = true
    error.value = ''
    notice.value = ''
    try {
      await api.reindexKnowledgeDocuments()
      await refresh()
      notice.value = `已重建 ${documents.value.length} 个文件的索引`
    } catch (reason) {
      error.value = reason?.message || String(reason)
    } finally {
      busy.value = false
    }
  }

  async function search() {
    error.value = ''
    if (!query.value.trim()) {
      results.value = []
      return
    }
    try {
      const items = await api.searchKnowledge(query.value.trim())
      results.value = Array.isArray(items) ? items : []
    } catch (reason) {
      error.value = reason?.message || String(reason)
    }
  }

  return { documents, busy, error, notice, answerMode, query, results, storagePath, refresh, importMarkdown, importDirectory, saveNote, remove, setAnswerMode, reindex, search }
})
