import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ModelProfileForm.vue', import.meta.url), 'utf8')

describe('ModelProfileForm thinking controls', () => {
  it('shows the thinking controls only for the interview profile', () => {
    expect(source).toContain('<div v-if="mode === \'interview\'" class="thinking-panel">')
    expect(source).toContain('v-model="profile.thinkingMode"')
    expect(source).toContain('value="disabled">关闭（实时推荐）')
    expect(source).toContain('value="auto">跟随模型默认')
    expect(source).toContain('value="enabled">开启')
  })

  it('offers bounded reasoning levels instead of a free-text field', () => {
    expect(source).toContain('v-model="profile.reasoningLevel"')
    for (const level of ['minimal', 'low', 'medium', 'high', 'xhigh', 'max']) {
      expect(source).toContain(`value="${level}"`)
    }
    expect(source).not.toContain('v-model.trim="profile.reasoningLevel"')
  })
})
