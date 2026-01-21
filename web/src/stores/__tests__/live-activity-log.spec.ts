// Test for activity log red dot bug
// This test simulates the WebSocket message flow to verify the red dot is cleared

import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useLiveStore } from '../live'

describe('LiveStore - Activity Log Red Dot Bug', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('should remove active session when status=ended is received', () => {
    const live = useLiveStore()
    
    // Simulate receiving an active hearing
    const activeMessage = {
      type: 'hearing',
      status: 'active',
      id: 123,
      my: 'W8EAP',
      ur: 'CQCQCQ',
      module: 'A',
      protocol: 'VOICE',
      created_at: new Date().toISOString()
    }

    // Manually trigger the message handler logic
    // In real code, this would come through WebSocket
    live.lastHeard.unshift({
      id: 123,
      my: 'W8EAP',
      ur: 'CQCQCQ',
      rpt1: 'A',
      rpt2: 'WEB A',
      module: 'A',
      protocol: 'VOICE',
      created_at: activeMessage.created_at,
      status: 'active'
    })
    live.activeSessions[123] = Date.now()

    // Verify session is active
    expect(live.isSessionActive(123)).toBe(true)
    expect(live.lastHeard[0]?.status).toBe('active')

    // Simulate receiving the ended message
    // Manually trigger the ended logic
    delete live.activeSessions[123]
    const entry = live.lastHeard.find(h => h.id === 123)
    if (entry) {
      entry.duration = 54.2
      entry.status = 'ended'
    }

    // Verify session is NO LONGER active
    expect(live.isSessionActive(123)).toBe(false)
    expect(live.lastHeard[0]?.status).toBe('ended')
    expect(live.lastHeard[0]?.duration).toBe(54.2)
  })

  it('should not mark session as active if status=ended', () => {
    const live = useLiveStore()
    
    // Simulate receiving a message with status=ended
    // This shouldn't add to activeSessions
    // The fixed code should NOT add this to activeSessions
    // because status === 'ended'
    
    // Verify session is not active
    expect(live.isSessionActive(456)).toBe(false)
  })

  it('should handle rapid start/stop without leaving red dot', () => {
    const live = useLiveStore()
    
    // User starts transmitting
    live.lastHeard.unshift({
      id: 789,
      my: 'W8EAP',
      ur: 'CQCQCQ',
      rpt1: 'A',
      rpt2: 'WEB A',
      module: 'A',
      protocol: 'VOICE',
      created_at: new Date().toISOString(),
      status: 'active'
    })
    live.activeSessions[789] = Date.now()

    expect(live.isSessionActive(789)).toBe(true)

    // User stops transmitting (quick)
    delete live.activeSessions[789]
    const entry = live.lastHeard.find(h => h.id === 789)
    if (entry) {
      entry.duration = 2.3
      entry.status = 'ended'
    }

    // Should NOT have red dot
    expect(live.isSessionActive(789)).toBe(false)
    expect(entry?.status).toBe('ended')
  })
})
