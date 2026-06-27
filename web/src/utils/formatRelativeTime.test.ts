import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { formatRelativeTime } from './formatRelativeTime'

const NOW = new Date('2026-06-27T12:00:00Z')

afterEach(() => { vi.useRealTimers() })

function ago(ms: number): string {
  return new Date(NOW.getTime() - ms).toISOString()
}

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(NOW)
  })

  it('returns "adesso" when less than 1 minute ago', () => {
    expect(formatRelativeTime(ago(30_000))).toBe('adesso')
  })

  it('returns "adesso" at exactly 0 ms ago', () => {
    expect(formatRelativeTime(ago(0))).toBe('adesso')
  })

  it('returns "1m fa" at exactly 1 minute ago', () => {
    expect(formatRelativeTime(ago(60_000))).toBe('1m fa')
  })

  it('returns "59m fa" just before the hour boundary', () => {
    expect(formatRelativeTime(ago(59 * 60_000))).toBe('59m fa')
  })

  it('returns "1h fa" at exactly 1 hour ago', () => {
    expect(formatRelativeTime(ago(60 * 60_000))).toBe('1h fa')
  })

  it('returns "3h fa" at 3 hours ago', () => {
    expect(formatRelativeTime(ago(3 * 60 * 60_000))).toBe('3h fa')
  })

  it('returns "23h fa" just before the day boundary', () => {
    expect(formatRelativeTime(ago(23 * 60 * 60_000))).toBe('23h fa')
  })

  it('returns "1g fa" at exactly 1 day ago', () => {
    expect(formatRelativeTime(ago(24 * 60 * 60_000))).toBe('1g fa')
  })

  it('returns "2g fa" at 2 days ago', () => {
    expect(formatRelativeTime(ago(2 * 24 * 60 * 60_000))).toBe('2g fa')
  })
})
