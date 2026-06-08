export function isDevOpsAdmin(accessToken: string | null): boolean {
  try {
    if (!accessToken) return false

    const parts = accessToken.split('.')
    if (parts.length !== 3) return false

    const payload = parts[1]
      .replaceAll('-', '+')
      .replaceAll('_', '/')
      .padEnd(Math.ceil(parts[1].length / 4) * 4, '=')

    const decoded: unknown = JSON.parse(atob(payload))

    if (
      decoded === null ||
      typeof decoded !== 'object' ||
      !('realm_access' in decoded)
    ) {
      return false
    }

    const realmAccess = (decoded as Record<string, unknown>)['realm_access']

    if (
      realmAccess === null ||
      typeof realmAccess !== 'object' ||
      !('roles' in realmAccess)
    ) {
      return false
    }

    const roles = (realmAccess as Record<string, unknown>)['roles']

    return Array.isArray(roles) && roles.includes('kubegate:devops-admin')
  } catch {
    return false
  }
}
