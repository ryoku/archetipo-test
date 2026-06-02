export async function apiFetch(
  path: string,
  token: string | null,
  init?: RequestInit,
): Promise<Response> {
  const headers = new Headers(init?.headers);

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(path, { ...init, headers });

  if (response.status === 401) {
    window.dispatchEvent(new Event('auth:unauthorized'));
  }

  return response;
}
