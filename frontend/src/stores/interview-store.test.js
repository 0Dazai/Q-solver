import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../services/api', () => ({
  api: {
    listInterviewSessions: vi.fn(async () => [
      { id: 'old-session', startedAt: '2026-07-29T05:12:00Z' },
    ]),
    getInterviewTimeline: vi.fn(async () => []),
  },
}))

import { useInterviewStore } from './interview'

describe('interview store session list', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('keeps the active session selectable before async persistence appears in the database list', async () => {
    const store = useInterviewStore()
    store.applyStatus({ sessionId: 'current-session', running: true })
    await store.refreshSessions()

    expect(store.selectedSessionId).toBe('current-session')
    expect(store.sessions[0]?.id).toBe('current-session')
  })
})
