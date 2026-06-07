import { describe, it, expect } from 'vitest'
import { isDevOpsAdmin } from './claims'

function makeToken(payload: object): string {
  const encoded = btoa(JSON.stringify(payload))
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
  return `header.${encoded}.signature`
}

describe('isDevOpsAdmin', () => {
  it('returns true for a valid JWT with kubegate:devops-admin in realm_access.roles', () => {
    const token = makeToken({
      realm_access: { roles: ['kubegate:devops-admin', 'other-role'] },
    })
    expect(isDevOpsAdmin(token)).toBe(true)
  })

  it('returns false for a valid JWT without kubegate:devops-admin', () => {
    const token = makeToken({
      realm_access: { roles: ['other-role', 'viewer'] },
    })
    expect(isDevOpsAdmin(token)).toBe(false)
  })

  it('returns false for null', () => {
    expect(isDevOpsAdmin(null)).toBe(false)
  })

  it('returns false for an empty string', () => {
    expect(isDevOpsAdmin('')).toBe(false)
  })

  it('returns false for a malformed token (not 3 parts)', () => {
    expect(isDevOpsAdmin('onlyone')).toBe(false)
    expect(isDevOpsAdmin('only.two')).toBe(false)
    expect(isDevOpsAdmin('too.many.parts.here')).toBe(false)
  })

  it('returns false for a token with a non-JSON payload', () => {
    const token = 'header.!!!invalid-base64!!!.signature'
    expect(isDevOpsAdmin(token)).toBe(false)
  })

  it('returns false for a token with no realm_access field', () => {
    const token = makeToken({ sub: 'user-123', email: 'user@example.com' })
    expect(isDevOpsAdmin(token)).toBe(false)
  })
})
