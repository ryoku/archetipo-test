import { UserManager, WebStorageStateStore } from 'oidc-client-ts'

export function createUserManager(): UserManager {
  const authority = import.meta.env.VITE_OIDC_ISSUER_URL as string
  const clientId = import.meta.env.VITE_OIDC_CLIENT_ID as string
  const redirectUri = `${window.location.origin}/auth/callback`
  const postLogoutRedirectUri = `${window.location.origin}/login`

  return new UserManager({
    authority,
    client_id: clientId,
    redirect_uri: redirectUri,
    post_logout_redirect_uri: postLogoutRedirectUri,
    scope: 'openid profile email',
    userStore: new WebStorageStateStore({ store: sessionStorage }),
  })
}
