import { defineStore } from 'pinia'
import { reactive, ref, computed, watch } from 'vue'
import { api } from '../services/api'
import { useUIStore } from './ui'
import { currentTheme, setTheme } from '../services/theme'
import { guessProviderFromBaseURL, PROVIDER_CATALOG } from '../utils/modelCapabilities'

export const useSettingsStore = defineStore('settings', () => {
  const ui = useUIStore()

  const settings = reactive({
    apiKey: '',
    provider: 'openai',
    baseURL: 'https://api.openai.com/v1',
    model: '',
    assistantModel: '',
    prompt: '',
    domainId: 'general-assistant',
    transparency: 0,
    keepContext: false,
    screenshotMode: 'window',
    resumePath: '',
    resumeContent: '',
    compressionQuality: 80,
    sharpening: 0,
    grayscale: true,
    noCompression: false,
    workMode: 'written',
    writtenModel: { provider: 'openai', model: '', apiKey: '', apiKeySet: false, baseURL: 'https://api.openai.com/v1', protocol: 'openai_chat_completions', thinkingMode: 'auto', reasoningLevel: '', disableResponseStorage: false, maxTokens: 0, temperature: 0, systemPrompt: '' },
    interviewModel: { provider: 'openai', model: '', apiKey: '', apiKeySet: false, baseURL: 'https://api.openai.com/v1', protocol: 'openai_chat_completions', thinkingMode: 'disabled', reasoningLevel: '', disableResponseStorage: false, maxTokens: 0, temperature: 0, systemPrompt: '' },
    transcription: { engine: 'auto', apiKey: '', apiKeySet: false, model: 'fun-asr-realtime-2025-09-15', endpoint: 'wss://dashscope.aliyuncs.com/api-ws/v1/inference', region: '', language: 'zh', hotwords: [], contextPhrases: [], vocabularyId: '', sentenceWaitMs: 1300, autoSubmit: true },
  })

  const tempSettings = reactive({ ...settings })
  const shortcuts = reactive({})
  const tempShortcuts = reactive({})
  const domainCategories = ref([])
  const resumeRawContent = ref('')
  const isResumeParsing = ref(false)
  const modelLists = reactive({ written: [], interview: [] })
  const modelLoading = reactive({ written: false, interview: false })
  const modelTesting = reactive({ written: false, interview: false })
  const modelConnections = reactive({ written: null, interview: null })
  const modeSwitching = ref(false)

  const isMacOS = ref(
    typeof navigator !== 'undefined' &&
    (navigator.platform.toLowerCase().includes('mac') ||
      navigator.userAgent.toLowerCase().includes('mac'))
  )

  const recordingAction = ref(null)
  const recordingText = ref('')
  const shortcutActions = [
    { action: 'screenshot', label: '截图', default: 'F8', macDefault: 'Cmd+1' },
    { action: 'send', label: '发送解题', default: 'Ctrl+J', macDefault: 'Cmd+J' },
    { action: 'cancel', label: '中断当前回答', default: 'Esc', macDefault: 'Esc' },
    { action: 'delete', label: '删除截图', default: 'Ctrl+D', macDefault: 'Cmd+D' },
    { action: 'toggle', label: '隐藏/显示', default: 'F9', macDefault: 'Cmd+2' },
    { action: 'minimize', label: '收起/恢复窗口', default: 'F7', macDefault: 'Cmd+4' },
    { action: 'clickthrough', label: '鼠标穿透', default: 'F10', macDefault: 'Cmd+3' },
    { action: 'move_up', label: '向上移动', default: 'Alt+Up', macDefault: 'Cmd+Option+Up' },
    { action: 'move_down', label: '向下移动', default: 'Alt+Down', macDefault: 'Cmd+Option+Down' },
    { action: 'move_left', label: '向左移动', default: 'Alt+Left', macDefault: 'Cmd+Option+Left' },
    { action: 'move_right', label: '向右移动', default: 'Alt+Right', macDefault: 'Cmd+Option+Right' },
    { action: 'scroll_up', label: '向上滚动', default: 'Alt+PgUp', macDefault: 'Cmd+Option+Shift+Up' },
    { action: 'scroll_down', label: '向下滚动', default: 'Alt+PgDn', macDefault: 'Cmd+Option+Shift+Down' },
    { action: 'mode_toggle', label: '切换笔试/面试模式', default: 'F6', macDefault: 'Cmd+4' },
    { action: 'interview_listening', label: '开始/暂停面试监听', default: 'F11', macDefault: 'Cmd+5' },
  ]

  const maskedKey = computed(() => {
    if (!settings.apiKey) return ''
    if (settings.apiKey.length < 8) return settings.apiKey
    return settings.apiKey.substring(0, 3) + '****' + settings.apiKey.substring(settings.apiKey.length - 4)
  })

  const solveShortcut = computed(() => shortcuts.screenshot?.keyName || shortcuts.solve?.keyName || 'F8')
  const sendShortcut = computed(() => shortcuts.send?.keyName || 'Ctrl+J')
  const deleteShortcut = computed(() => shortcuts.delete?.keyName || 'Ctrl+D')
  const toggleShortcut = computed(() => shortcuts.toggle?.keyName || 'F9')
  const minimizeShortcut = computed(() => shortcuts.minimize?.keyName || 'F7')
  const cancelShortcut = computed(() => shortcuts.cancel?.keyName || 'Esc')

  const statusText = ref('就绪')
  const statusIcon = ref('●')

  function resetStatus() {
    if (!settings.writtenModel.apiKeySet) {
      statusText.value = '未配置'
      statusIcon.value = '!'
    } else {
      statusText.value = '已连接'
      statusIcon.value = '✓'
    }
  }

  watch(() => settings.writtenModel.apiKeySet, () => resetStatus(), { immediate: true })

  watch(() => tempSettings.transparency, (newVal) => {
    applyTransparency(1.0 - newVal)
  })

  window.addEventListener('theme-changed', () => {
    requestAnimationFrame(() => {
      applyTransparency(1.0 - (tempSettings.transparency ?? 0))
    })
  })

  watch(() => resumeRawContent.value, (newVal) => {
    tempSettings.resumeContent = newVal || ''
  })

  let themeInitialized = false
  watch(currentTheme, () => {
    if (!themeInitialized) {
      themeInitialized = true
      return
    }
    saveSettingsSilent()
  })

  function applyTransparency(opacity) {
    const root = document.documentElement
    if (!root) return
    const style = getComputedStyle(root)
    const base = style.getPropertyValue('--surface-base').trim()
    let r, g, b
    if (base.startsWith('rgba')) {
      const parts = base.match(/rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*[\d.]+\s*\)/)
      if (parts) {
        r = parts[1]
        g = parts[2]
        b = parts[3]
      }
    }
    if (!r) {
      const theme = root.getAttribute('data-theme')
      if (theme === 'light') {
        r = '252'
        g = '252'
        b = '255'
      } else {
        r = '10'
        g = '10'
        b = '14'
      }
    }
    root.style.setProperty('--app-bg-r', r)
    root.style.setProperty('--app-bg-g', g)
    root.style.setProperty('--app-bg-b', b)
    root.style.setProperty('--app-bg-a', String(opacity))
    root.style.setProperty('--app-panel-a', String(Math.max(0, opacity * 0.5)))
    root.style.setProperty('--text-shadow-a', String(Math.max(0, (1 - opacity) * 0.8)))
  }

  async function loadSettings() {
    try {
      const [backendConfig, categories] = await Promise.all([
        api.getSettings(),
        api.getDomainCategories(),
      ])
      if (categories) domainCategories.value = categories
      applyConfig(backendConfig)
      if (backendConfig.shortcuts) Object.assign(shortcuts, backendConfig.shortcuts)
      if (backendConfig.theme) setTheme(backendConfig.theme)
      if ((settings.writtenModel.apiKey || settings.writtenModel.apiKeySet) && (!settings.writtenModel.model || settings.writtenModel.model === 'auto')) {
        await fetchModels(settings.writtenModel)
        if (modelLists.written.length > 0 && !settings.model) {
          settings.writtenModel.model = modelLists.written[0]
          tempSettings.writtenModel.model = modelLists.written[0]
        }
      }
    } catch (e) {
      console.error('loadSettings error', e)
    }
  }

  function applyConfig(config) {
    settings.apiKey = ''
    settings.provider = config.writtenModel?.provider || config.provider || guessProviderFromBaseURL(config.baseURL)
    settings.baseURL = config.writtenModel?.baseURL || config.baseURL || 'https://api.openai.com/v1'
    settings.model = config.writtenModel?.model || config.model || ''
    settings.assistantModel = config.assistantModel || ''
    settings.prompt = config.prompt || ''
    settings.domainId = config.domainId || 'general-assistant'
    settings.compressionQuality = config.compressionQuality || 80
    settings.sharpening = config.sharpening || 0
    settings.grayscale = config.grayscale !== undefined ? config.grayscale : true
    settings.noCompression = config.noCompression || false
    settings.keepContext = config.keepContext || false
    settings.resumePath = config.resumePath || ''
    settings.resumeContent = config.resumeContent || ''
    settings.screenshotMode = config.screenshotMode || 'window'
    settings.workMode = config.workMode || 'written'
    settings.writtenModel = {
      ...settings.writtenModel,
      ...(config.writtenModel || {}),
      provider: config.writtenModel?.provider || config.provider || 'openai',
      model: config.writtenModel?.model || config.model || '',
      apiKey: '',
      apiKeySet: Boolean(config.writtenModel?.apiKeySet),
      baseURL: config.writtenModel?.baseURL || config.baseURL || 'https://api.openai.com/v1',
    }
    settings.interviewModel = { ...settings.interviewModel, ...(config.interviewModel || {}), apiKey: '', apiKeySet: Boolean(config.interviewModel?.apiKeySet) }
    settings.transcription = { ...settings.transcription, ...(config.transcription || {}), apiKey: '', apiKeySet: Boolean(config.transcription?.apiKeySet) }

    const opacity = config.opacity !== undefined ? config.opacity : 1.0
    settings.transparency = 1.0 - opacity
    applyTransparency(opacity)
    Object.assign(tempSettings, JSON.parse(JSON.stringify(settings)))
  }

  async function fetchModels(profile = tempSettings.writtenModel, mode = 'written') {
    const apiKey = profile.apiKey
    if (!apiKey && !profile.apiKeySet) {
      ui.showToast('请先填写并保存该模型的 API Key', 'warning')
      return false
    }
    modelLoading[mode] = true
    try {
      const baseURL = profile.baseURL
      const provider = profile.provider || guessProviderFromBaseURL(baseURL)
      const models = await api.getProfileModels(mode, apiKey, baseURL, provider)
      if (models && models.length > 0) {
        modelLists[mode] = models
        if (!profile.model) {
          profile.model = models[0]
        }
        return true
      }
      modelLists[mode] = []
      return false
    } catch (e) {
      console.error('获取模型列表失败', e)
      modelLists[mode] = []
      let errorMsg = '获取模型列表失败'
      try {
        const errObj = JSON.parse(e.message || e)
        if (errObj.message) errorMsg = errObj.message
      } catch (_) {
        errorMsg = e.message || '获取模型列表失败'
      }
      const definition = PROVIDER_CATALOG[profile.provider || guessProviderFromBaseURL(profile.baseURL)]
      if (definition?.modelListing === 'manual') {
        errorMsg = `${definition.label} 不提供标准模型列表，请手动输入模型 ID。`
      }
      ui.showToast(errorMsg, 'error')
      return false
    } finally {
      modelLoading[mode] = false
    }
  }

  async function refreshModels(mode = 'written') {
    const profile = mode === 'interview' ? tempSettings.interviewModel : tempSettings.writtenModel
    const success = await fetchModels(profile, mode)
    if (success && modelLists[mode].length > 0) {
      ui.showToast(`已加载 ${modelLists[mode].length} 个模型`, 'success')
    }
  }

  async function testConnection(mode = 'written') {
    const profile = mode === 'interview' ? tempSettings.interviewModel : tempSettings.writtenModel
    if (!profile.model) {
      ui.showToast('请先选择模型', 'warning')
      return
    }
    modelTesting[mode] = true
    modelConnections[mode] = null
    try {
      const result = await api.testModelProfile(mode, profile.apiKey, profile.baseURL, profile.provider || guessProviderFromBaseURL(profile.baseURL), profile.model, profile.protocol, profile.thinkingMode, profile.reasoningLevel)
      if (result === '') {
        modelConnections[mode] = { type: 'success', icon: '✓', message: `模型 ${profile.model} 连接成功` }
        ui.showToast('连接测试成功', 'success')
      } else {
        modelConnections[mode] = { type: 'error', icon: '!', message: result }
        ui.showToast('连接测试失败', 'error')
      }
    } catch (e) {
      modelConnections[mode] = { type: 'error', icon: '!', message: e.message || '连接测试失败' }
    } finally {
      modelTesting[mode] = false
    }
  }

  async function testTranscription() {
    const asr = tempSettings.transcription
    ui.isTestingConnection = true
    try {
      const result = await api.testTranscriptionConnection(asr.apiKey, asr.model, asr.endpoint, asr.language, asr.engine)
      ui.showToast(result || (asr.engine === 'windows' || (!asr.apiKey && !asr.apiKeySet) ? 'Windows 语音识别可用' : '千问连接测试成功'), result ? 'error' : 'success')
    } catch (e) {
      ui.showToast(e.message || '千问连接测试失败', 'error')
    } finally {
      ui.isTestingConnection = false
    }
  }

  function buildConfigToSave(sourceSettings, sourceShortcuts) {
    return {
      apiKey: '',
      provider: sourceSettings.writtenModel.provider,
      baseURL: sourceSettings.writtenModel.baseURL,
      model: sourceSettings.writtenModel.model,
      assistantModel: sourceSettings.assistantModel,
      prompt: sourceSettings.writtenModel.systemPrompt || sourceSettings.prompt || '',
      domainId: sourceSettings.domainId,
      opacity: 1.0 - sourceSettings.transparency,
      keepContext: sourceSettings.keepContext,
      screenshotMode: sourceSettings.screenshotMode,
      compressionQuality: sourceSettings.compressionQuality,
      sharpening: sourceSettings.sharpening,
      grayscale: sourceSettings.grayscale,
      noCompression: sourceSettings.noCompression,
      resumePath: sourceSettings.resumePath,
      resumeContent: sourceSettings.resumeContent,
      shortcuts: sourceShortcuts,
      theme: currentTheme.value,
      workMode: sourceSettings.workMode,
      writtenModel: { ...sourceSettings.writtenModel },
      interviewModel: { ...sourceSettings.interviewModel },
      transcription: { ...sourceSettings.transcription, hotwords: splitTerms(sourceSettings.transcription.hotwords), contextPhrases: splitTerms(sourceSettings.transcription.contextPhrases) },
    }
  }

  function splitTerms(value) {
    if (Array.isArray(value)) return value.filter(Boolean)
    return String(value || '').split(/[\n,，]/).map(v => v.trim()).filter(Boolean)
  }

  async function saveSettings() {
    try {
      if (!tempSettings.writtenModel.model && (tempSettings.writtenModel.apiKey || tempSettings.writtenModel.apiKeySet)) {
        ui.showToast('正在自动获取模型...', 'info')
        await fetchModels(tempSettings.writtenModel)
        if (!tempSettings.writtenModel.model && modelLists.written.length > 0) {
          tempSettings.writtenModel.model = modelLists.written[0]
        }
      }

      Object.assign(shortcuts, JSON.parse(JSON.stringify(tempShortcuts)))

      const err = await api.syncSettings(JSON.stringify(buildConfigToSave(tempSettings, tempShortcuts)))
      if (err) {
        ui.showToast(err, 'error')
      } else {
        const writtenKeySet = settings.writtenModel.apiKeySet || Boolean(tempSettings.writtenModel.apiKey)
        const interviewKeySet = settings.interviewModel.apiKeySet || Boolean(tempSettings.interviewModel.apiKey)
        const transcriptionKeySet = settings.transcription.apiKeySet || Boolean(tempSettings.transcription.apiKey)
        ui.showToast('设置已保存', 'success')
        Object.assign(settings, tempSettings)
        settings.writtenModel.apiKey = ''
        settings.interviewModel.apiKey = ''
        settings.transcription.apiKey = ''
        settings.writtenModel.apiKeySet = writtenKeySet
        settings.interviewModel.apiKeySet = interviewKeySet
        settings.transcription.apiKeySet = transcriptionKeySet
        resetStatus()
        closeSettings()
      }
    } catch (e) {
      console.error('保存设置失败', e)
      ui.showToast('保存失败', 'error')
    }
  }

  function openSettings() {
    api.restoreFocus()
    Object.assign(tempSettings, JSON.parse(JSON.stringify(settings)))
    Object.assign(tempShortcuts, JSON.parse(JSON.stringify(shortcuts)))
    modelConnections.written = null
    modelConnections.interview = null
    if (settings.resumeContent) resumeRawContent.value = settings.resumeContent
    ui.showSettings = true
  }

  function closeSettings() {
    if (isResumeParsing.value) {
      ui.showToast('简历正在解析中，请稍候...', 'warning')
      return
    }
    api.removeFocus()
    ui.showSettings = false
    if (recordingAction.value) api.stopRecordingKey()
    recordingAction.value = null
    recordingText.value = ''
    resetTempSettings()
  }

  function resetTempSettings() {
    Object.assign(tempSettings, settings)
    applyTransparency(1.0 - settings.transparency)
  }

  function applyWorkMode(mode) {
    settings.workMode = mode === 'interview' ? 'interview' : 'written'
    tempSettings.workMode = settings.workMode
  }

  async function setWorkMode(mode) {
    const target = mode === 'interview' ? 'interview' : 'written'
    if (modeSwitching.value || settings.workMode === target) return
    modeSwitching.value = true
    try {
      await api.restoreFocus()
      const error = await api.setWorkMode(target)
      if (error) ui.showToast(error, 'error')
    } catch (error) {
      ui.showToast(error?.message || '切换工作模式失败', 'error')
    } finally {
      modeSwitching.value = false
      await api.removeFocus()
    }
  }

  function recordKey(action) {
    if (isMacOS.value) return
    recordingAction.value = action
    recordingText.value = '请按键...'
    api.startRecordingKey(action)
  }

  async function selectResume() {
    const path = await api.selectResume()
    if (path) {
      tempSettings.resumePath = path
      resumeRawContent.value = ''
      ui.showToast('已选择文件，正在自动解析...', 'info')
      await parseResume()
    }
  }

  async function clearResume() {
    await api.clearResume()
    tempSettings.resumePath = ''
    resumeRawContent.value = ''
  }

  async function parseResume() {
    if (!tempSettings.resumePath) return
    isResumeParsing.value = true
    try {
      const result = await api.parseResume()
      resumeRawContent.value = result
      tempSettings.resumeContent = result
      ui.showToast('简历解析成功', 'success')
    } catch (e) {
      console.error(e)
      ui.showToast(e?.message || '简历解析失败', 'error')
    } finally {
      isResumeParsing.value = false
    }
  }

  async function saveSettingsSilent() {
    try {
      await api.syncSettings(JSON.stringify(buildConfigToSave(settings, shortcuts)))
    } catch (e) {
      console.error('静默保存失败', e)
    }
  }

  return {
    settings, tempSettings,
    shortcuts, tempShortcuts,
    domainCategories, modelLists, modelLoading, modelTesting, modelConnections, modeSwitching,
    resumeRawContent, isResumeParsing,
    isMacOS,
    recordingAction, recordingText, shortcutActions,
    maskedKey, solveShortcut, sendShortcut, deleteShortcut, toggleShortcut, minimizeShortcut, cancelShortcut,
    statusText, statusIcon, resetStatus,
    loadSettings, fetchModels, refreshModels, testConnection, testTranscription,
    saveSettings, saveSettingsSilent, openSettings, closeSettings, resetTempSettings,
    recordKey, selectResume, clearResume, parseResume,
    applyTransparency, applyWorkMode, setWorkMode,
  }
})
