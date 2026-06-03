import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { apiFetch } from './client';

function makeFetchStub(status: number): ReturnType<typeof vi.fn> {
  return vi.fn().mockResolvedValue(new Response(null, { status }));
}

describe('apiFetch', () => {
  let dispatchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    dispatchSpy = vi.spyOn(window, 'dispatchEvent');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('appends Authorization header when a token is provided', async () => {
    const fetchStub = makeFetchStub(200);
    vi.stubGlobal('fetch', fetchStub);

    await apiFetch('/api/v1/products', 'my-token');

    const [, init] = fetchStub.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(headers.get('Authorization')).toBe('Bearer my-token');
  });

  it('does not set Authorization header when token is null', async () => {
    const fetchStub = makeFetchStub(200);
    vi.stubGlobal('fetch', fetchStub);

    await apiFetch('/api/v1/products', null);

    const [, init] = fetchStub.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(headers.get('Authorization')).toBeNull();
  });

  it('dispatches auth:unauthorized on window when the response status is 401', async () => {
    vi.stubGlobal('fetch', makeFetchStub(401));

    await apiFetch('/api/v1/products', 'my-token');

    expect(dispatchSpy).toHaveBeenCalledOnce();
    const dispatched = dispatchSpy.mock.calls[0][0] as Event;
    expect(dispatched.type).toBe('auth:unauthorized');
  });

  it('does not dispatch auth:unauthorized for non-401 responses', async () => {
    vi.stubGlobal('fetch', makeFetchStub(403));

    await apiFetch('/api/v1/products', 'my-token');

    expect(dispatchSpy).not.toHaveBeenCalled();
  });

  it('returns the Response in all cases', async () => {
    const response = new Response(null, { status: 200 });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response));

    const result = await apiFetch('/api/v1/products', null);

    expect(result).toBe(response);
  });
});
