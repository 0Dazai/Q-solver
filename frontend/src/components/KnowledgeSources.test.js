import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./KnowledgeSources.vue', import.meta.url), 'utf8')

describe('KnowledgeSources', () => {
  it('uses a closed native disclosure by default', () => {
    expect(source).toContain('<details v-if="items.length" class="knowledge-sources">')
    expect(source).toContain('<summary class="sources-summary">')
    expect(source).not.toMatch(/<details[^>]*\sopen(?:\s|=|>)/)
  })

  it('keeps the source count and both toggle labels visible in the template contract', () => {
    expect(source).toContain('{{ items.length }} 条')
    expect(source).toContain('expand-label">展开')
    expect(source).toContain('collapse-label">收起')
  })
})
