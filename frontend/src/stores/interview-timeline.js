export function normalizeTimelineMessage(message) {
  return {
    messageId: message?.messageId || message?.id || '',
    sessionId: message?.sessionId || '',
    turnId: message?.turnId || '',
    questionId: message?.questionId || '',
    role: message?.role || '',
    content: message?.content || '',
    status: message?.status || 'final',
    createdAt: message?.createdAt || new Date().toISOString(),
  }
}

export function mergeTimelineEvent(messages, rawEvent) {
  const event = normalizeTimelineMessage(rawEvent)
  if (!event.messageId) return messages
  const index = messages.findIndex(item => item.messageId === event.messageId)
  if (index < 0) return [...messages, event]
  const next = [...messages]
  const current = next[index]
  next[index] = {
    ...current,
    ...event,
    content: rawEvent?.append ? `${current.content || ''}${event.content}` : event.content,
  }
  return next
}

export function shouldFollowBottom(scrollTop, clientHeight, scrollHeight) {
  return scrollHeight - scrollTop - clientHeight <= 48
}

export function boundTimelineWindow(messages, limit = 1000) {
  if (!Array.isArray(messages) || messages.length <= limit) return messages
  return messages.slice(messages.length - limit)
}
