import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { api } from '../services/api'
import { useUIStore } from './ui'
import { acceptsSessionEvent } from './interview-session'
import { boundTimelineWindow, mergeTimelineEvent, normalizeTimelineMessage } from './interview-timeline'

export const useInterviewStore = defineStore('interview', () => {
  const ui = useUIStore()
  const status = reactive({
    running: false, systemAudio: 'stopped', microphone: false, asr: 'disconnected',
    droppedPackets: 0, queuedPackets: 0, partial: '', candidate: '', submitted: '',
    suppression: '', answering: false, lastError: '',
  })
  const transcript = ref([])
  const localTranscript = ref([])
  const localPartial = ref('')
  const history = ref([])
  const timeline = ref([])
  const sessions = ref([])
  const hasOlderMessages = ref(true)
  const loadingHistory = ref(false)
  const answer = ref('')
  const sources = ref([])
  const currentSessionId = ref('')
  const selectedSessionId = ref('')
  const editing = ref(false)
  const editedQuestion = ref('')
  const starting = ref(false)
  const stopping = ref(false)
  let lastToggleAt = 0

  const listeningLabel = computed(() => {
    if (starting.value) return '正在启动'
    if (status.running && status.asr === 'connected') return '正在监听'
    if (status.asr === 'reconnecting') return '正在重连'
    return '未监听'
  })

  function applyStatus(next) {
    if (!next) return
    if (next.sessionId && next.sessionId !== currentSessionId.value) {
      const previousActiveSessionId = currentSessionId.value
      currentSessionId.value = next.sessionId
      sessions.value = ensureActiveSession(sessions.value, next.sessionId)
      if (!selectedSessionId.value || selectedSessionId.value === previousActiveSessionId) {
        selectedSessionId.value = next.sessionId
        loadTimeline(next.sessionId, true)
      }
      refreshSessions()
    }
    const wasAnswering = status.answering
    Object.assign(status, next)
    if (wasAnswering && !status.answering) {
      timeline.value = timeline.value.map(msg => {
        if (msg.role === 'ai_suggestion' && msg.status === 'streaming') {
          return { ...msg, status: 'final' }
        }
        return msg
      })
    }
  }

  function acceptTimeline(event) {
    if (!event) return
    if (!selectedSessionId.value) selectedSessionId.value = event.sessionId || ''
    if (!acceptsSessionEvent(selectedSessionId.value, event)) return
    timeline.value = boundTimelineWindow(mergeTimelineEvent(timeline.value, event))
  }

  async function refreshSessions() {
    try {
      const items = await api.listInterviewSessions(50)
      sessions.value = ensureActiveSession(Array.isArray(items) ? items : [], currentSessionId.value)
    } catch (_) {}
  }

  async function loadTimeline(sessionId = selectedSessionId.value, reset = false) {
    if (!sessionId || loadingHistory.value) return
    loadingHistory.value = true
    try {
      const before = reset || !timeline.value.length ? '' : timeline.value[0]?.createdAt || ''
      const items = await api.getInterviewTimeline(sessionId, before, 50)
      const normalized = (Array.isArray(items) ? items : []).map(normalizeTimelineMessage)
      timeline.value = boundTimelineWindow(reset ? normalized : [...normalized, ...timeline.value])
      hasOlderMessages.value = normalized.length === 50
    } catch (error) {
      status.lastError = error?.message || '读取面试历史失败'
    } finally {
      loadingHistory.value = false
    }
  }

  async function selectSession(sessionId) {
    selectedSessionId.value = sessionId
    await loadTimeline(sessionId, true)
  }

  async function openMarkdown() {
    if (!selectedSessionId.value) return
    try { await api.openInterviewMarkdown(selectedSessionId.value) } catch (error) {
      ui.showToast(error?.message || '打开面试记录失败', 'error')
    }
  }

  async function openRecordDirectory() {
    try { await api.openInterviewRecordDirectory() } catch (error) {
      ui.showToast(error?.message || '打开记录目录失败', 'error')
    }
  }

  function acceptTranscript(event) {
    if (!event?.text || !acceptsSessionEvent(currentSessionId.value, event)) return
    transcript.value = [...transcript.value.slice(-19), event]
  }

  function acceptLocalTranscript(event) {
    if (!event?.text || !acceptsSessionEvent(currentSessionId.value, event)) return
    if (event.kind === 'partial') {
      localPartial.value = event.text
      return
    }
    localPartial.value = ''
    localTranscript.value = [...localTranscript.value.slice(-19), event]
  }

  function acceptAnswer(chunk) {
    if (!acceptsSessionEvent(currentSessionId.value, chunk)) return
    const text = typeof chunk === 'string' ? chunk : chunk?.text
    if (!text) return
    const questionId = typeof chunk === 'object' ? chunk?.questionId : ''
    const item = history.value.find(entry => entry.questionId === questionId) || history.value[history.value.length - 1]
    if (item) item.answer += text
    answer.value += text
  }

  function acceptQuestion(question) {
    if (!question || !acceptsSessionEvent(currentSessionId.value, question)) return
    if (question.submitted) {
      status.submitted = question.correctedText || ''
      answer.value = ''
      const questionId = question.questionId || `question-${Date.now()}`
      if (!history.value.some(item => item.questionId === questionId)) {
        history.value = [...history.value.slice(-29), { questionId, question: question.correctedText || '', answer: '' }]
      }
    }
    if (question.correctedText) status.candidate = question.correctedText
  }

  function acceptError(message) {
    status.lastError = typeof message === 'string' ? message : '面试会话发生错误'
  }

  function acceptSources(payload) {
    if (!payload || !acceptsSessionEvent(currentSessionId.value, payload)) return
    sources.value = Array.isArray(payload.items) ? payload.items : []
  }

  async function refreshStatus() {
    try { applyStatus(await api.getInterviewStatus()) } catch (_) {}
  }

  async function start() {
    starting.value = true
    status.lastError = ''
    try {
      const error = await api.startInterviewSession()
      if (error) status.lastError = error
      await refreshStatus()
    } finally {
      starting.value = false
    }
  }

  async function stop() {
    if (stopping.value) return
    stopping.value = true
    try {
      await api.stopInterviewSession()
      await refreshStatus()
      status.suppression = '监听已暂停'
    } catch (error) {
      status.lastError = error?.message || '暂停监听失败'
    } finally {
      stopping.value = false
    }
  }

  async function toggleListening() {
    const now = Date.now()
    if (starting.value || stopping.value || now - lastToggleAt < 400) return
    lastToggleAt = now
    const wasRunning = status.running
    if (wasRunning) stopping.value = true
    else starting.value = true
    status.lastError = ''
    try {
      const error = await api.toggleInterviewListening()
      if (error) {
        status.lastError = error
        ui.showToast(error, 'error')
      } else {
        ui.showToast(wasRunning ? '监听已暂停' : '监听已启动', 'success')
      }
      await refreshStatus()
    } finally {
      starting.value = false
      stopping.value = false
    }
  }

  function cancelAnswer() { api.cancelInterviewAnswer() }
  function beginEdit() { editedQuestion.value = status.candidate || status.submitted || ''; editing.value = true }
  function cancelEdit() { editing.value = false }
  async function submitEdit() {
    if (!editedQuestion.value.trim()) return
    await api.editInterviewQuestion(editedQuestion.value.trim())
    await api.submitInterviewQuestion()
    editing.value = false
  }
  async function resubmit() {
    if (!status.submitted) return
    await api.editInterviewQuestion(status.submitted)
    await api.submitInterviewQuestion()
  }
  async function deepen() {
    if (!status.submitted) return
    await api.editInterviewQuestion(`${status.submitted}\n\n请进一步展开，补充原理、边界条件、示例和面试表达建议。`)
    await api.submitInterviewQuestion()
  }

  return {
    status, transcript, localTranscript, localPartial, history, timeline, sessions, hasOlderMessages, loadingHistory,
    answer, sources, currentSessionId, selectedSessionId, editing, editedQuestion, starting, stopping, listeningLabel,
    applyStatus, acceptTranscript, acceptLocalTranscript, acceptAnswer, acceptQuestion, acceptTimeline, acceptSources, acceptError,
    refreshSessions, loadTimeline, selectSession, openMarkdown, openRecordDirectory,
    refreshStatus, start, stop, toggleListening, cancelAnswer, beginEdit, cancelEdit, submitEdit, resubmit, deepen,
  }
})

function ensureActiveSession(items, activeSessionId) {
  if (!activeSessionId || items.some(item => item?.id === activeSessionId)) return items
  return [{ id: activeSessionId, startedAt: '', active: true }, ...items]
}
