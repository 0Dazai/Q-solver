import { describe, expect, it } from 'vitest'
import { acceptsSessionEvent } from './interview-session'

describe('acceptsSessionEvent', () => {
  it('rejects late events from an old interview session', () => {
    expect(acceptsSessionEvent('current', { sessionId: 'old' })).toBe(false)
    expect(acceptsSessionEvent('current', { sessionId: 'current' })).toBe(true)
  })

  it('rejects unscoped events once a current session exists', () => {
    expect(acceptsSessionEvent('current', { text: 'chunk' })).toBe(false)
  })
})
