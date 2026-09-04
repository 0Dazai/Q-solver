export function acceptsSessionEvent(currentSessionId, event) {
  const eventSessionId = event?.sessionId
  if (!currentSessionId) return true
  return Boolean(eventSessionId) && currentSessionId === eventSessionId
}
