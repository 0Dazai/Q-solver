<template>
  <div v-if="ui.showSettings" class="modal" id="settings-modal" style="display: flex">
    <div class="modal-content">
      <div class="modal-warning-banner">
        <Icon name="alert-triangle" :size="13" class="banner-icon" />
        <span>当前窗口已获得焦点，关闭设置后将自动恢复低打扰模式</span>
      </div>

      <div class="modal-header">
        <div class="tabs">
          <div class="tab" :class="{ active: ui.activeTab === 'general' }" @click="ui.activeTab = 'general'">常规</div>
          <div class="tab" :class="{ active: ui.activeTab === 'api' }" @click="ui.activeTab = 'api'">模型连接</div>
          <div class="tab" :class="{ active: ui.activeTab === 'screenshot' }" @click="ui.activeTab = 'screenshot'">截图</div>
          <div class="tab" :class="{ active: ui.activeTab === 'resume' }" @click="ui.activeTab = 'resume'">简历</div>
          <div class="tab" :class="{ active: ui.activeTab === 'knowledge' }" @click="ui.activeTab = 'knowledge'">资料库</div>
          <div class="tab" :class="{ active: ui.activeTab === 'transcription' }" @click="ui.activeTab = 'transcription'">语音转写</div>
        </div>
        <button class="close-btn" @click="settingsStore.closeSettings">
          <Icon name="x" :size="16" />
        </button>
      </div>

      <div class="modal-body">
        <div v-show="ui.activeTab === 'general'" class="tab-pane">
          <div class="form-group">
            <label>工作模式</label>
            <div class="mode-control">
              <label><input v-model="settingsStore.tempSettings.workMode" type="radio" value="written" /> 笔试模式</label>
              <label><input v-model="settingsStore.tempSettings.workMode" type="radio" value="interview" /> 面试模式</label>
            </div>
          </div>
          <div class="form-group">
            <label>快捷键配置 {{ settingsStore.isMacOS ? '(macOS 使用固定快捷键)' : '(点击录制)' }}</label>
            <div class="shortcut-list">
              <div class="shortcut-item" v-for="key in settingsStore.shortcutActions" :key="key.action">
                <span>{{ key.label }}</span>
                <button
                  class="btn-record"
                  :class="{ recording: settingsStore.recordingAction === key.action, disabled: settingsStore.isMacOS }"
                  @click="!settingsStore.isMacOS && settingsStore.recordKey(key.action)"
                >
                  {{
                    settingsStore.recordingAction === key.action
                      ? settingsStore.recordingText
                      : (settingsStore.tempShortcuts[key.action]?.keyName || (settingsStore.isMacOS ? key.macDefault : key.default))
                  }}
                </button>
              </div>
            </div>
          </div>

          <div class="form-group">
            <label for="opacity-slider">窗口透明度 <span>{{ Math.round(settingsStore.tempSettings.transparency * 100) }}%</span></label>
            <input
              id="opacity-slider"
              v-model.number="settingsStore.tempSettings.transparency"
              type="range"
              min="0.0"
              max="1.0"
              step="0.05"
            />
          </div>
        </div>

        <div v-show="ui.activeTab === 'api'" class="tab-pane model-tab">
          <div class="form-group">
            <label>编辑的模型连接</label>
            <div class="mode-control">
              <label><input v-model="apiProfileMode" type="radio" value="written" /> 笔试模型连接</label>
              <label><input v-model="apiProfileMode" type="radio" value="interview" /> 面试模型连接</label>
            </div>
          </div>
          <ModelProfileForm v-if="apiProfileMode === 'written'" :profile="settingsStore.tempSettings.writtenModel" mode="written" label="笔试" :models="settingsStore.modelLists.written" :loading="settingsStore.modelLoading.written" :testing="settingsStore.modelTesting.written" :connection-status="settingsStore.modelConnections.written" @refresh="settingsStore.refreshModels('written')" @test="settingsStore.testConnection('written')" />
          <ModelProfileForm v-else :profile="settingsStore.tempSettings.interviewModel" mode="interview" label="面试" :models="settingsStore.modelLists.interview" :loading="settingsStore.modelLoading.interview" :testing="settingsStore.modelTesting.interview" :connection-status="settingsStore.modelConnections.interview" @refresh="settingsStore.refreshModels('interview')" @test="settingsStore.testConnection('interview')" />
          <div v-if="apiProfileMode === 'written'" class="form-group domain-group">
            <DomainSelector v-model="settingsStore.tempSettings.domainId" :categories="settingsStore.domainCategories" />
          </div>
        </div>

        <div v-show="ui.activeTab === 'screenshot'" class="tab-pane">
          <ScreenshotSettings v-model="screenshotConfig" />
        </div>

        <div v-if="ui.activeTab === 'resume'" class="tab-pane">
          <ResumeImport
            :resumePath="settingsStore.tempSettings.resumePath"
            :rawContent="settingsStore.resumeRawContent"
            :isParsing="settingsStore.isResumeParsing"
            @update:rawContent="val => settingsStore.resumeRawContent = val"
            @select-resume="settingsStore.selectResume"
            @clear-resume="settingsStore.clearResume"
            @parse-resume="settingsStore.parseResume"
          />
        </div>

        <div v-if="ui.activeTab === 'knowledge'" class="tab-pane">
          <KnowledgeSettings />
        </div>

        <div v-show="ui.activeTab === 'transcription'" class="tab-pane model-tab">
          <div class="form-group"><label>转写引擎</label><select v-model="settingsStore.tempSettings.transcription.engine" class="manual-model-input boxed-input"><option value="auto">自动（优先千问，无 Key 时 Windows）</option><option value="dashscope">千问实时 ASR</option><option value="windows">Windows 系统语音识别</option></select></div>
          <div class="form-group"><label>千问 API Key <span v-if="settingsStore.tempSettings.transcription.apiKeySet" class="saved-key">已安全保存</span></label><input v-model="settingsStore.tempSettings.transcription.apiKey" class="manual-model-input boxed-input" type="password" autocomplete="off" :placeholder="settingsStore.tempSettings.transcription.apiKeySet ? '留空保持当前 Key，输入内容将替换' : '保存后不会回传到前端'" /></div>
          <p class="hint-text">未配置千问 Key 时，面试模式使用本机 Windows 语音识别；仅支持已安装的中文（简体）系统识别器。</p>
          <div class="form-group"><label>模型名</label><input v-model.trim="settingsStore.tempSettings.transcription.model" class="manual-model-input boxed-input" /></div>
          <div class="form-group"><label>区域或服务地址</label><input v-model.trim="settingsStore.tempSettings.transcription.endpoint" class="manual-model-input boxed-input" /></div>
          <div class="profile-grid"><div class="form-group"><label>区域</label><input v-model.trim="settingsStore.tempSettings.transcription.region" class="manual-model-input boxed-input" placeholder="可选" /></div><div class="form-group"><label>语言</label><input v-model.trim="settingsStore.tempSettings.transcription.language" class="manual-model-input boxed-input" /></div><div class="form-group"><label>句末等待 (ms)</label><input v-model.number="settingsStore.tempSettings.transcription.sentenceWaitMs" class="manual-model-input boxed-input" type="number" min="300" max="10000" /></div></div>
          <div class="form-group"><label>百炼热词表 ID</label><input v-model.trim="settingsStore.tempSettings.transcription.vocabularyId" class="manual-model-input boxed-input" placeholder="可选：已在百炼创建的 vocabulary_id" /></div>
          <div class="form-group"><label>热词备注</label><textarea v-model="settingsStore.tempSettings.transcription.hotwords" class="extra-prompt-input" placeholder="每行或逗号分隔；可作为本地配置说明"></textarea></div>
          <div class="form-group"><label>上下文词表</label><textarea v-model="settingsStore.tempSettings.transcription.contextPhrases" class="extra-prompt-input" placeholder="每行或逗号分隔"></textarea></div>
          <p class="hint-text">默认模型仅支持语言提示和百炼热词表 ID；上下文词表会在支持 context 的 Fun-ASR 实时模型中发送。</p>
          <label class="inline-check"><input v-model="settingsStore.tempSettings.transcription.autoSubmit" type="checkbox" /> 自动提交稳定问题</label>
          <button class="btn-secondary" :disabled="ui.isTestingConnection" @click="settingsStore.testTranscription">测试连接</button>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn-primary" @click="settingsStore.saveSettings">保存</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useUIStore } from '../stores/ui'
import { useSettingsStore } from '../stores/settings'
import ResumeImport from './ResumeImport.vue'
import KnowledgeSettings from './KnowledgeSettings.vue'
import ModelProfileForm from './ModelProfileForm.vue'
import DomainSelector from './DomainSelector.vue'
import ScreenshotSettings from './ScreenshotSettings.vue'
import Icon from './Icon.vue'

const ui = useUIStore()
const settingsStore = useSettingsStore()
const apiProfileMode = ref('written')

const screenshotConfig = computed({
  get: () => ({
    compressionQuality: settingsStore.tempSettings.compressionQuality,
    sharpening: settingsStore.tempSettings.sharpening,
    grayscale: settingsStore.tempSettings.grayscale,
    noCompression: settingsStore.tempSettings.noCompression,
    screenshotMode: settingsStore.tempSettings.screenshotMode,
  }),
  set: (val) => {
    settingsStore.tempSettings.compressionQuality = val.compressionQuality
    settingsStore.tempSettings.sharpening = val.sharpening
    settingsStore.tempSettings.grayscale = val.grayscale
    settingsStore.tempSettings.noCompression = val.noCompression
    settingsStore.tempSettings.screenshotMode = val.screenshotMode
  }
})
</script>

<style scoped>
.modal {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  pointer-events: auto;
}

.modal-content {
  width: 680px;
  max-width: 92vw;
  height: 580px;
  max-height: 85vh;
  background: var(--surface-popover);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: modalIn 0.25s var(--ease-out);
}

@keyframes modalIn {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.modal-warning-banner {
  background: var(--warning-bg);
  border: 1px solid var(--warning-border);
  border-radius: var(--radius-full);
  padding: 6px 16px;
  color: var(--color-warning);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--sp-1-5);
  margin: 12px auto 4px auto;
  width: fit-content;
}

.banner-icon { flex-shrink: 0; }

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sp-3) var(--sp-5);
  border-bottom: 1px solid var(--border-subtle);
}

.tabs {
  display: flex;
  gap: var(--sp-1);
  flex-wrap: nowrap;
  overflow-x: auto;
  scrollbar-width: thin;
}

.tab {
  padding: var(--sp-2) var(--sp-4);
  border-radius: var(--radius-sm);
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--duration-fast) ease;
}

.tab:hover {
  color: var(--text-primary);
  background: var(--surface-card);
}

.tab.active {
  color: var(--accent);
  background: var(--accent-muted);
}

.close-btn {
  color: var(--text-muted);
  cursor: pointer;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  border: none;
  background: transparent;
  transition: all var(--duration-fast) ease;
}

.close-btn:hover {
  background: var(--surface-card-hover);
  color: var(--text-primary);
}

.modal-body {
  flex: 1;
  overflow: hidden;
  padding: 0;
  min-height: 0;
  position: relative;
}

.tab-pane {
  height: 100%;
  overflow-y: auto;
  padding: var(--sp-5);
  box-sizing: border-box;
}

.api-tab-pane {
  overflow-y: auto;
  display: block;
}

.modal-footer {
  padding: var(--sp-4) var(--sp-5);
  border-top: 1px solid var(--border-subtle);
  display: flex;
  justify-content: flex-end;
}

.btn-primary {
  padding: var(--sp-2) var(--sp-6);
  border-radius: var(--radius-md);
  background: var(--accent);
  color: var(--text-inverse);
  font-size: var(--text-sm);
  font-weight: 700;
  border: none;
  cursor: pointer;
  transition: all var(--duration-fast) ease;
}

.btn-primary:hover {
  background: var(--accent-hover);
  transform: translateY(-1px);
}

.form-group { margin-bottom: var(--sp-5); }

.form-group label {
  display: block;
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: var(--sp-2);
}

.model-tab {
  height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
}

.model-select-group { margin-bottom: var(--sp-5); }

.manual-model-shell {
  margin-top: var(--sp-2);
  display: flex;
  align-items: center;
  min-height: 38px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background: var(--surface-input);
}

.manual-model-input {
  width: 100%;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-primary);
  font-size: var(--text-sm);
  padding: 0 var(--sp-3);
}

.boxed-input {
  box-sizing: border-box;
  min-height: 38px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--surface-input);
  padding: 0 var(--sp-3);
}

select.boxed-input { width: 100%; color: var(--text-primary); }
.profile-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--sp-2); }
.profile-grid .form-group { min-width: 0; }
.mode-control { display: flex; gap: var(--sp-4); color: var(--text-secondary); font-size: var(--text-sm); }
.mode-control label, .inline-check { display: flex; align-items: center; gap: var(--sp-2); }
.inline-check { margin-bottom: var(--sp-4); color: var(--text-secondary); font-size: var(--text-sm); }
.btn-secondary { padding: var(--sp-2) var(--sp-4); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-card); color: var(--text-primary); cursor: pointer; }
.btn-secondary:disabled { opacity: .5; cursor: not-allowed; }

.manual-model-input::placeholder {
  color: var(--text-muted);
}

.domain-group {
  display: block;
  margin-bottom: var(--sp-5);
}

.extra-prompt-group {
  margin-bottom: 0;
}

.extra-prompt-input {
  width: 100%;
  min-height: 70px;
  resize: vertical;
  box-sizing: border-box;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--surface-input);
  color: var(--text-primary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  padding: var(--sp-3);
  outline: none;
  font-family: var(--font-sans);
}

.extra-prompt-input:focus {
  border-color: var(--accent);
  box-shadow: var(--shadow-glow);
}

.extra-prompt-input::placeholder {
  color: var(--text-muted);
}

:deep(.domain-selector) {
  height: auto;
  display: flex;
  flex-direction: column;
}

:deep(.domain-list) {
  max-height: 180px;
}

.model-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--sp-2);
}

.model-actions {
  display: flex;
  gap: var(--sp-2);
}

.btn-icon {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all var(--duration-fast) ease;
}

.btn-icon:hover {
  background: var(--surface-card-hover);
  color: var(--text-primary);
  border-color: var(--border-hover);
}

.btn-icon:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.action-icon {
  width: 14px;
  height: 14px;
}

.action-icon.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.connection-status {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  padding: var(--sp-2) var(--sp-3);
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  margin-top: var(--sp-2);
}

.connection-status.success {
  background: var(--success-bg);
  border: 1px solid var(--success-border);
  color: var(--color-success);
}

.connection-status.error {
  background: var(--error-bg);
  border: 1px solid var(--error-border);
  color: var(--color-error);
}

.cs-icon {
  font-size: 14px;
  font-weight: 700;
}

.cs-text { font-weight: 600; }

.hint-text {
  font-size: var(--text-xs);
  color: var(--text-muted);
  margin-top: var(--sp-2);
}

.warning-hint { color: var(--color-warning); }

.shortcut-list {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--sp-2);
}

.shortcut-item:last-child:nth-child(odd) { grid-column: 1 / -1; }

.shortcut-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--sp-2) var(--sp-3);
  background: var(--surface-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  font-size: var(--text-sm);
  color: var(--text-primary);
  gap: var(--sp-2);
}

.btn-record {
  padding: var(--sp-1) var(--sp-3);
  font-size: var(--text-xs);
  font-family: var(--font-mono);
  font-weight: 600;
  background: var(--surface-input);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-xs);
  color: var(--text-secondary);
  cursor: pointer;
  min-width: 70px;
  text-align: center;
  transition: all var(--duration-fast) ease;
}

.btn-record:hover:not(.disabled) {
  border-color: var(--accent-border);
  color: var(--accent);
}

.btn-record.recording {
  border-color: var(--accent);
  background: var(--accent-muted);
  color: var(--accent);
  animation: pulseRecord 1s ease-in-out infinite;
}

.btn-record.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@keyframes pulseRecord {
  0%, 100% { box-shadow: 0 0 0 0 var(--accent-glow); }
  50% { box-shadow: 0 0 0 4px var(--accent-glow); }
}

input[type="range"] {
  width: 100%;
  height: 4px;
  appearance: none;
  -webkit-appearance: none;
  background: var(--border-default);
  border-radius: var(--radius-full);
  outline: none;
  cursor: pointer;
}

input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--surface-elevated);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
}
</style>
