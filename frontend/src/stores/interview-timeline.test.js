import { describe, expect, it } from 'vitest'
import { boundTimelineWindow, mergeTimelineEvent, shouldFollowBottom } from './interview-timeline'

describe('interview timeline', () => {
  it('merges streamed AI chunks into one message', () => {
    let items = mergeTimelineEvent([], { messageId: 'a1', role: 'ai_suggestion', content: '我', append: true })
    items = mergeTimelineEvent(items, { messageId: 'a1', role: 'ai_suggestion', content: '负责音频', append: true })
    expect(items).toHaveLength(1)
    expect(items[0].content).toBe('我负责音频')
  })

  it('keeps candidate history when a new question arrives', () => {
    const items = [{ messageId: 'c1', role: 'candidate', content: '我的旧回答' }]
    const next = mergeTimelineEvent(items, { messageId: 'q2', role: 'interviewer', content: '下一题' })
    expect(next[0].content).toBe('我的旧回答')
    expect(next).toHaveLength(2)
  })

  it('only follows when near the bottom', () => {
    expect(shouldFollowBottom(952, 500, 1500)).toBe(true)
    expect(shouldFollowBottom(700, 500, 1500)).toBe(false)
  })

  it('bounds the in-memory window while local history remains pageable', () => {
    const items = Array.from({ length: 1005 }, (_, index) => ({ messageId: `${index}` }))
    const bounded = boundTimelineWindow(items)
    expect(bounded).toHaveLength(1000)
    expect(bounded[0].messageId).toBe('5')
  })
})
