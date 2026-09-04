<template>
  <main class="interview-view">
    <section class="interview-header">
      <div>
        <p class="eyebrow">面试模式</p>
        <h1>{{ interview.listeningLabel }}</h1>
        <p class="subtle">系统音频 {{ audioText }}<span v-if="interview.status.microphone"> · 已检测到本机讲话</span></p>
      </div>
      <div class="header-actions">
        <button type="button" class="listener-button" @click="showKnowledge = !showKnowledge">
          <span>{{ showKnowledge ? '收起资料库' : '资料库' }}</span>
        </button>
        <button v-if="!interview.status.running" type="button" class="listener-button primary" :disabled="interview.starting || interview.stopping" @click="interview.toggleListening">
          <Icon name="play" :size="15" />
          <span>{{ interview.starting ? '正在启动' : '开始监听' }}</span>
        </button>
        <button v-else type="button" class="listener-button" :disabled="interview.starting || interview.stopping" @click="interview.toggleListening">
          <Icon name="square" :size="14" />
          <span>{{ interview.stopping ? '正在暂停' : '暂停监听' }}</span>
        </button>
        <button class="icon-button" title="取消当前回答" :disabled="!interview.status.answering" @click="interview.cancelAnswer">
          <Icon name="x" :size="18" />
        </button>
      </div>
    </section>

    <section class="workspace">
      <div class="transcript-panel">
        <div class="panel-title"><span>实时识别</span><span class="badge" :class="interview.status.asr">{{ asrText }}</span></div>
        <div class="live-block">
          <strong>面试官正在说</strong>
          <p v-if="interview.status.partial" class="partial">{{ interview.status.partial }}</p>
          <p v-else class="empty">完整问题会进入右侧面试时间线。</p>
        </div>
        <div class="live-block local-transcript">
          <strong>我正在回答</strong>
          <p v-if="interview.localPartial" class="local-partial">{{ interview.localPartial }}</p>
          <p v-else class="empty">麦克风 final 会永久保存到对应轮次。</p>
        </div>
        <p v-if="interview.status.suppression" class="suppression">{{ interview.status.suppression }}</p>
        <div class="profile-ready" :class="{ ready: interview.status.running }">
          <span class="profile-dot"></span>
          {{ interview.status.running ? '候选人资料已预载' : '开始监听时预载简历与资料' }}
        </div>
      </div>

      <div class="answer-panel">
        <div class="panel-title timeline-title">
          <span>面试时间线</span>
          <div class="record-actions">
            <select v-if="interview.sessions.length" :value="interview.selectedSessionId" aria-label="选择面试记录" @change="interview.selectSession($event.target.value)">
              <option v-for="session in interview.sessions" :key="session.id" :value="session.id">
                {{ formatSession(session) }}
              </option>
            </select>
            <button class="text-button" @click="interview.openMarkdown">打开 Markdown</button>
            <button class="text-button" @click="interview.openRecordDirectory">记录目录</button>
            <button class="text-button" :disabled="!interview.status.candidate && !interview.status.submitted" @click="interview.beginEdit">编辑问题</button>
          </div>
        </div>
        <template v-if="interview.editing">
          <textarea v-model="interview.editedQuestion" class="question-editor" aria-label="编辑问题"></textarea>
          <div class="edit-actions"><button class="text-button" @click="interview.cancelEdit">取消</button><button class="text-button accent" @click="interview.submitEdit">提交</button></div>
        </template>
        <div ref="timelineEl" class="timeline-scroll" aria-label="面试问答历史" @scroll="onTimelineScroll">
          <button v-if="interview.hasOlderMessages" class="load-older" :disabled="interview.loadingHistory" @click="loadOlder">
            {{ interview.loadingHistory ? '正在读取…' : '上滑读取更早记录' }}
          </button>
          <p v-if="!interview.timeline.length" class="timeline-empty">等待面试官问题、AI 参考答案和你的实际回答。</p>
          <article v-for="message in interview.timeline" :key="message.messageId" class="timeline-message" :class="message.role">
            <div class="message-role">{{ roleLabel(message.role) }}</div>
            <div class="message-content">{{ message.content || '正在生成…' }}<span v-if="message.status === 'streaming' && message.role === 'ai_suggestion'" class="stream-caret"></span></div>
          </article>
        </div>
        <button v-if="hasNewMessages" class="new-message-button" @click="scrollToBottom">有新消息，回到底部</button>
        <div class="answer-actions">
          <button class="text-button" :disabled="!interview.status.submitted" @click="interview.resubmit">重新提交</button>
          <button class="text-button" :disabled="!interview.status.submitted" @click="interview.deepen">深入分析</button>
        </div>
        <KnowledgeSources :items="interview.sources" />
      </div>
    </section>

    <p v-if="interview.status.lastError" class="error-message">{{ interview.status.lastError }}</p>
    <div v-if="showKnowledge" class="drawer-backdrop" @click.self="showKnowledge = false">
      <aside class="knowledge-drawer">
        <div class="drawer-title">
          <div><strong>面试资料库</strong><span>独立于 PDF 简历解析</span></div>
          <button type="button" class="icon-button" title="关闭资料库" @click="showKnowledge = false">
            <Icon name="x" :size="18" />
          </button>
        </div>
        <KnowledgeLibrary />
      </aside>
    </div>
  </main>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import Icon from './Icon.vue'
import KnowledgeLibrary from './KnowledgeLibrary.vue'
import KnowledgeSources from './KnowledgeSources.vue'
import { useInterviewStore } from '../stores/interview'

const interview = useInterviewStore()
const showKnowledge = ref(false)
const timelineEl = ref(null)
const followingBottom = ref(true)
const hasNewMessages = ref(false)
const audioText = computed(() => interview.status.systemAudio === 'running' ? '已连接' : interview.status.systemAudio === 'starting' ? '启动中' : '未连接')
const asrText = computed(() => {
  if (interview.status.asr === 'connected') return interview.status.engine === 'windows' ? 'Windows 语音识别' : '千问 ASR 已连接'
  return ({ connecting: 'ASR 连接中', reconnecting: 'ASR 重连中', error: 'ASR 异常' }[interview.status.asr] || 'ASR 未连接')
})

function roleLabel(role) {
  return ({ interviewer: '面试官', ai_suggestion: 'AI 参考', candidate: '我的回答' })[role] || '消息'
}

function formatSession(session) {
  const date = session?.startedAt ? new Date(session.startedAt) : null
  return date && !Number.isNaN(date.getTime())
    ? date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    : '当前面试'
}

function onTimelineScroll() {
  const el = timelineEl.value
  if (!el) return
  followingBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight <= 48
  if (followingBottom.value) hasNewMessages.value = false
}

async function scrollToBottom() {
  await nextTick()
  const el = timelineEl.value
  if (el) el.scrollTop = el.scrollHeight
  followingBottom.value = true
  hasNewMessages.value = false
}

async function loadOlder() {
  const el = timelineEl.value
  const oldHeight = el?.scrollHeight || 0
  await interview.loadTimeline(interview.selectedSessionId, false)
  await nextTick()
  if (el) el.scrollTop += el.scrollHeight - oldHeight
}

watch(
  () => interview.timeline.map(item => `${item.messageId}:${item.content.length}:${item.status}`).join('|'),
  async () => {
    if (followingBottom.value) await scrollToBottom()
    else hasNewMessages.value = true
  },
)
</script>

<style scoped>
.interview-view { position: relative; flex: 1; min-height: 0; min-width: 0; width: min(1120px, calc(100% - 32px)); margin: 0 auto; display: flex; flex-direction: column; gap: var(--sp-3); padding: var(--sp-3) 0 var(--sp-4); overflow-x: hidden; pointer-events: auto; }
.interview-header { display: flex; justify-content: space-between; align-items: center; gap: var(--sp-4); padding: var(--sp-4); background: var(--surface-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); }
.eyebrow { margin: 0 0 4px; color: var(--accent); font-size: var(--text-xs); font-weight: var(--weight-semibold); }
h1 { margin: 0; color: var(--text-primary); font-size: var(--text-xl); letter-spacing: 0; }
.subtle, .empty { margin: 5px 0 0; color: var(--text-secondary); font-size: var(--text-sm); }
.header-actions, .answer-actions, .edit-actions { display: flex; align-items: center; gap: var(--sp-2); }
.listener-button, .icon-button { height: 34px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-card); color: var(--text-primary); cursor: pointer; }
.listener-button { gap: 6px; padding: 0 12px; font-size: var(--text-sm); font-weight: var(--weight-semibold); }
.listener-button.primary { background: var(--accent); border-color: var(--accent); color: var(--text-inverse); }
.icon-button { width: 34px; }
.listener-button:disabled, .icon-button:disabled, .text-button:disabled { opacity: .45; cursor: not-allowed; }
.workspace { min-height: 0; min-width: 0; flex: 1; display: grid; grid-template-columns: minmax(240px, .8fr) minmax(300px, 1.2fr); gap: var(--sp-3); }
.transcript-panel, .answer-panel { min-height: 0; overflow: auto; padding: var(--sp-4); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface-card); }
.answer-panel { display: flex; flex-direction: column; }
.panel-title { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-2); color: var(--text-primary); font-size: var(--text-sm); font-weight: var(--weight-bold); }
.badge { font-size: var(--text-xs); font-weight: var(--weight-medium); color: var(--text-muted); }
.badge.connected { color: var(--color-success); }.badge.reconnecting { color: var(--color-warning); }.badge.error { color: var(--color-error); }
.partial { color: var(--accent); font-size: var(--text-sm); line-height: 1.6; margin: var(--sp-3) 0; }
.live-block { margin-top: var(--sp-4); padding: var(--sp-3); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-input); }
.live-block strong { color: var(--text-primary); font-size: var(--text-xs); }
.suppression { margin: var(--sp-3) 0 0; color: var(--color-warning); font-size: var(--text-xs); }
.local-transcript { margin-top: var(--sp-3); }.local-partial { margin: var(--sp-2) 0 0; color: var(--text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.profile-ready { display: flex; align-items: center; gap: 7px; margin-top: var(--sp-4); color: var(--text-muted); font-size: var(--text-xs); }.profile-ready.ready { color: var(--color-success); }.profile-dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }
.answer-actions { margin-top: var(--sp-3); padding-top: var(--sp-3); border-top: 1px solid var(--border-subtle); }
.timeline-title { align-items: flex-start; }.record-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: var(--sp-2); }.record-actions select { max-width: 138px; height: 26px; border: 1px solid var(--border-default); border-radius: 6px; background: var(--surface-input); color: var(--text-secondary); font-size: var(--text-xs); }
.timeline-scroll { position: relative; flex: 1; min-height: 180px; overflow-y: auto; overscroll-behavior: contain; margin-top: var(--sp-3); padding-right: 4px; scroll-behavior: smooth; }
.timeline-empty { margin: var(--sp-8) auto; max-width: 280px; color: var(--text-muted); text-align: center; font-size: var(--text-sm); line-height: 1.6; }
.timeline-message { max-width: 88%; margin: 0 0 var(--sp-3); padding: var(--sp-3); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface-input); }
.timeline-message.interviewer { margin-right: auto; border-left: 3px solid var(--text-muted); }.timeline-message.ai_suggestion { margin-left: auto; border-right: 3px solid var(--accent); }.timeline-message.candidate { margin-left: auto; border-right: 3px solid var(--color-success); }
.message-role { margin-bottom: 6px; color: var(--text-muted); font-size: var(--text-xs); font-weight: var(--weight-semibold); }.ai_suggestion .message-role { color: var(--accent); }.candidate .message-role { color: var(--color-success); }
.message-content { color: var(--text-primary); font-size: var(--text-sm); line-height: 1.65; white-space: pre-wrap; word-break: break-word; }.stream-caret { display: inline-block; width: 5px; height: 14px; margin-left: 3px; vertical-align: -2px; background: var(--accent); animation: blink 1s steps(2) infinite; }
.load-older { display: block; margin: 0 auto var(--sp-3); border: 0; background: transparent; color: var(--text-muted); font-size: var(--text-xs); cursor: pointer; }.new-message-button { align-self: center; margin-top: -36px; z-index: 2; border: 1px solid var(--accent); border-radius: 999px; padding: 6px 12px; background: var(--surface-elevated); color: var(--accent); font-size: var(--text-xs); cursor: pointer; box-shadow: var(--shadow-sm); }
@keyframes blink { 50% { opacity: 0; } }
.text-button { border: 0; padding: 3px 0; background: transparent; color: var(--text-secondary); font: inherit; font-size: var(--text-xs); cursor: pointer; }.text-button:hover:not(:disabled) { color: var(--text-primary); }.text-button.accent { color: var(--accent); }
.question-editor { width: 100%; min-height: 90px; box-sizing: border-box; margin: var(--sp-3) 0; padding: var(--sp-3); resize: vertical; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-primary); font: inherit; line-height: 1.5; }.edit-actions { justify-content: flex-end; margin-bottom: var(--sp-3); }
.error-message { margin: 0; padding: var(--sp-2) var(--sp-3); color: var(--color-error); background: var(--error-bg); border: 1px solid var(--error-border); border-radius: var(--radius-sm); font-size: var(--text-xs); }
.drawer-backdrop { position: absolute; inset: 0; z-index: 30; display: flex; justify-content: flex-end; background: rgba(20, 25, 40, .16); border-radius: var(--radius-md); }
.knowledge-drawer { width: min(420px, calc(100% - 24px)); min-width: 0; display: flex; flex-direction: column; gap: var(--sp-3); padding: var(--sp-3); box-sizing: border-box; background: var(--surface-elevated); border-left: 1px solid var(--border-default); box-shadow: var(--shadow-lg); }
.drawer-title { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-3); }
.drawer-title strong, .drawer-title span { display: block; }.drawer-title strong { color: var(--text-primary); }.drawer-title span { margin-top: 2px; color: var(--text-muted); font-size: var(--text-xs); }
.knowledge-drawer .knowledge-library { flex: 1; }
@media (max-width: 680px) { .interview-view { width: min(100% - 20px, 1120px); }.interview-header { align-items: flex-start; }.header-actions { flex-wrap: wrap; justify-content: flex-end; }.workspace { grid-template-columns: 1fr; overflow-y: auto; }.transcript-panel { min-height: 180px; }.answer-panel { min-height: 240px; } }
</style>
