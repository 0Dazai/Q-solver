<template>
  <section class="profile-form">
    <div class="form-group model-select-group">
      <div class="model-header">
        <label>{{ label }}模型</label>
        <div class="model-actions">
          <button class="btn-icon" :disabled="loading" title="刷新模型列表" @click="$emit('refresh')"><Icon name="refresh" :size="15" :spinning="loading" /></button>
          <button class="btn-icon" :disabled="testing || !profile.model" title="测试模型连通性" @click="$emit('test')"><Icon :name="testing ? 'loader' : 'play'" :size="15" :spinning="testing" /></button>
        </div>
      </div>
      <ModelSelect v-model="profile.model" :models="models" :loading="loading" />
      <div class="manual-model-shell"><input v-model.trim="profile.model" class="manual-model-input" placeholder="手动输入模型 ID 或接入点 ID" /></div>
    </div>

    <div class="form-group"><label>模型提供商</label><select v-model="profile.provider" class="boxed-input" @change="applyProviderDefaults"><option v-for="(item, code) in PROVIDER_CATALOG" :key="code" :value="code">{{ item.label }}</option></select><p v-if="providerHint" class="field-hint">{{ providerHint }}</p></div>
    <div class="form-group"><label>独立 API Key <span class="saved" v-if="profile.apiKeySet">已安全保存</span></label><input v-model="profile.apiKey" class="manual-model-input boxed-input" type="password" autocomplete="off" :placeholder="profile.apiKeySet ? '留空保持当前 Key，输入内容将替换' : '输入该模型的 API Key'" /></div>
    <div class="form-group"><label>Base URL</label><input v-model.trim="profile.baseURL" class="manual-model-input boxed-input" placeholder="https://api.openai.com/v1" /></div>
    <div class="form-group"><label>接口协议</label><select v-model="profile.protocol" class="boxed-input"><option v-for="protocol in protocolOptions" :key="protocol" :value="protocol">{{ protocolLabel(protocol) }}</option></select></div>
    <label v-if="profile.protocol === 'openai_responses'" class="inline-check"><input v-model="profile.disableResponseStorage" type="checkbox" /> 不保存 Responses 请求结果</label>
    <div v-if="mode === 'interview'" class="thinking-panel">
      <div class="thinking-heading"><div><strong>思考模式</strong><p>{{ thinkingHint }}</p></div><span class="mode-badge">{{ thinkingBadge }}</span></div>
      <div class="profile-grid thinking-grid">
        <div class="form-group"><label>模式</label><select v-model="profile.thinkingMode" class="boxed-input"><option value="disabled">关闭（实时推荐）</option><option value="auto">跟随模型默认</option><option value="enabled">开启</option></select></div>
        <div class="form-group"><label>思考强度</label><select v-model="profile.reasoningLevel" class="boxed-input" :disabled="profile.thinkingMode !== 'enabled'"><option value="">模型默认</option><option value="minimal">极低</option><option value="low">低</option><option value="medium">中</option><option value="high">高</option><option value="xhigh">很高</option><option value="max">最高</option></select></div>
        <div class="form-group"><label>Max Tokens</label><input v-model.number="profile.maxTokens" class="manual-model-input boxed-input" type="number" min="0" /></div>
      </div>
    </div>
    <div class="form-group"><label>Temperature</label><input v-model.number="profile.temperature" class="manual-model-input boxed-input" type="number" min="0" max="2" step="0.1" /></div>
    <div class="form-group"><label>系统提示词</label><textarea v-model="profile.systemPrompt" class="extra-prompt-input" :placeholder="mode === 'written' ? '例如：编程题默认使用 Java 解答。' : '例如：以简洁、准确的面试回答方式作答。'"></textarea></div>
    <div v-if="connectionStatus" class="connection-status" :class="connectionStatus.type"><span class="cs-icon">{{ connectionStatus.icon }}</span><span class="cs-text">{{ connectionStatus.message }}</span></div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import Icon from './Icon.vue'
import ModelSelect from './ModelSelect.vue'
import { PROVIDER_CATALOG } from '../utils/modelCapabilities'

const props = defineProps({ profile: { type: Object, required: true }, mode: { type: String, required: true }, label: { type: String, required: true }, models: { type: Array, default: () => [] }, loading: Boolean, testing: Boolean, connectionStatus: { type: Object, default: null } })
defineEmits(['refresh', 'test'])

const selectedProvider = computed(() => PROVIDER_CATALOG[props.profile.provider] || PROVIDER_CATALOG.custom)
const protocolOptions = computed(() => selectedProvider.value.protocols)
const providerHint = computed(() => selectedProvider.value.hint || '')
const thinkingBadge = computed(() => ({ disabled: '已关闭', enabled: '已开启', auto: '自动' }[props.profile.thinkingMode] || '自动'))
const thinkingHint = computed(() => {
  if (props.mode === 'interview' && props.profile.thinkingMode === 'disabled') return '实时技术面试优先首句速度；“深入分析”仍会临时启用中等思考。'
  if (props.profile.thinkingMode === 'enabled') return '适合复杂算法、架构设计和代码调试，首句等待会增加。'
  return '不发送强制开关，由当前模型决定；部分混合模型默认会思考。'
})

function applyProviderDefaults() {
  const provider = selectedProvider.value
  if (provider.baseURL) props.profile.baseURL = provider.baseURL
  if (!provider.protocols.includes(props.profile.protocol)) props.profile.protocol = provider.protocols[0]
}

function protocolLabel(protocol) {
  return protocol === 'openai_responses' ? 'OpenAI Responses' : 'OpenAI Chat Completions'
}
</script>

<style scoped>
.profile-form { display: block; }.form-group { margin-bottom: var(--sp-5); }.form-group label { display: block; margin-bottom: var(--sp-2); color: var(--text-primary); font-size: var(--text-sm); font-weight: 600; }.field-hint { margin: var(--sp-2) 0 0; color: var(--text-muted); font-size: var(--text-xs); line-height: 1.5; }.saved { margin-left: var(--sp-2); color: var(--color-success); font-size: var(--text-xs); font-weight: var(--weight-medium); }.model-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--sp-2); }.model-actions { display: flex; gap: var(--sp-2); }.btn-icon { width: 30px; height: 30px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-card); color: var(--text-secondary); cursor: pointer; }.btn-icon:disabled { opacity: .45; cursor: not-allowed; }.manual-model-shell { display: flex; align-items: center; min-height: 38px; margin-top: var(--sp-2); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-input); }.manual-model-input { width: 100%; min-height: 38px; box-sizing: border-box; border: 0; outline: 0; background: transparent; color: var(--text-primary); font: inherit; font-size: var(--text-sm); padding: 0 var(--sp-3); }.boxed-input { box-sizing: border-box; width: 100%; min-height: 38px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-primary); padding: 0 var(--sp-3); }.boxed-input:disabled { opacity: .55; cursor: not-allowed; }.inline-check { display:flex; align-items:center; gap:var(--sp-2); margin:-8px 0 var(--sp-4); color:var(--text-secondary); font-size:var(--text-xs); }.thinking-panel { margin-bottom: var(--sp-5); padding: var(--sp-3); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: color-mix(in srgb, var(--surface-card) 82%, var(--color-primary) 4%); }.thinking-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--sp-3); margin-bottom:var(--sp-3); }.thinking-heading strong { color:var(--text-primary); font-size:var(--text-sm); }.thinking-heading p { margin:4px 0 0; color:var(--text-muted); font-size:var(--text-xs); line-height:1.45; }.mode-badge { flex:none; padding:3px 8px; border-radius:999px; color:var(--color-primary); background:color-mix(in srgb, var(--color-primary) 12%, transparent); font-size:var(--text-xs); font-weight:600; }.profile-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--sp-2); }.profile-grid .form-group { min-width: 0; }.thinking-grid .form-group { margin-bottom:0; }.extra-prompt-input { width: 100%; min-height: 70px; box-sizing: border-box; resize: vertical; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-input); color: var(--text-primary); padding: var(--sp-3); font: inherit; font-size: var(--text-sm); line-height: 1.5; }.connection-status { display: flex; gap: var(--sp-2); padding: var(--sp-2) var(--sp-3); border-radius: var(--radius-sm); font-size: var(--text-xs); }.connection-status.success { color: var(--color-success); background: var(--success-bg); }.connection-status.error { color: var(--color-error); background: var(--error-bg); }@media(max-width:480px){.profile-grid{grid-template-columns:1fr;}.thinking-grid .form-group{margin-bottom:var(--sp-3);}.thinking-grid .form-group:last-child{margin-bottom:0;}}
</style>
