import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import {
  apiClient,
  buildObjectPath,
  buildMetaPath,
  errorMessage,
  setTokenGetter,
  getToken,
  downloadObject,
} from './client';
import { AxiosError, AxiosHeaders } from 'axios';

describe('buildObjectPath', () => {
  it('keeps slashes as path separators but encodes each segment', () => {
    expect(buildObjectPath('my-bucket', 'a/b/c.txt')).toBe(
      '/api/v1/buckets/my-bucket/objects/a/b/c.txt',
    );
  });

  it('encodes spaces, hash, question mark, percent and unicode within segments', () => {
    expect(buildObjectPath('b', 'dir name/weird #?%.txt')).toBe(
      '/api/v1/buckets/b/objects/dir%20name/weird%20%23%3F%25.txt',
    );
    // Multibyte UTF-8: accented Latin "é" (2 bytes) plus the euro sign "€" (3 bytes).
    expect(buildObjectPath('b', 'café€.txt')).toBe(
      '/api/v1/buckets/b/objects/caf%C3%A9%E2%82%AC.txt',
    );
    // Astral-plane emoji (surrogate pair, 4 bytes in UTF-8).
    expect(buildObjectPath('b', '🚀.txt')).toBe(
      '/api/v1/buckets/b/objects/%F0%9F%9A%80.txt',
    );
  });

  it('encodes the bucket name', () => {
    expect(buildObjectPath('weird bucket', 'k')).toBe(
      '/api/v1/buckets/weird%20bucket/objects/k',
    );
  });

});

describe('buildMetaPath', () => {
  it('uses the /meta/{key} prefix route (not /objects/{key}/meta)', () => {
    expect(buildMetaPath('b', 'a/b')).toBe('/api/v1/buckets/b/meta/a/b');
  });

  it('encodes bucket and each key segment', () => {
    expect(buildMetaPath('weird bucket', 'dir name/café€.txt')).toBe(
      '/api/v1/buckets/weird%20bucket/meta/dir%20name/caf%C3%A9%E2%82%AC.txt',
    );
  });
});

describe('errorMessage', () => {
  it('returns the backend message envelope when present', () => {
    const headers = new AxiosHeaders();
    const err = new AxiosError('boom', 'ERR', undefined, undefined, {
      status: 502,
      data: { error: 'upstream_error', message: 'failed to list objects' },
      statusText: 'Bad Gateway',
      headers,
      config: { headers },
    });
    expect(errorMessage(err, 'fallback')).toBe('failed to list objects');
  });

  it('falls back when error is not an axios error', () => {
    expect(errorMessage(new Error('x'), 'fallback')).toBe('fallback');
    expect(errorMessage('nope', 'fallback')).toBe('fallback');
  });
});

describe('token getter wiring', () => {
  afterEach(() => {
    setTokenGetter(() => null);
  });

  it('returns null before a getter is wired', () => {
    setTokenGetter(() => null);
    expect(getToken()).toBeNull();
  });

  it('reflects the wired getter', () => {
    setTokenGetter(() => 'abc123');
    expect(getToken()).toBe('abc123');
  });
});

describe('request interceptor', () => {
  afterEach(() => setTokenGetter(() => null));

  it('attaches a Bearer header from the token getter', async () => {
    setTokenGetter(() => 'tok');
    const handlers = (apiClient.interceptors.request as unknown as {
      handlers: Array<{ fulfilled: (c: { headers: Record<string, unknown> }) => unknown }>;
    }).handlers;
    const config = { headers: {} as Record<string, unknown> };
    const result = (await handlers[0].fulfilled(config)) as { headers: Record<string, unknown> };
    expect(result.headers.Authorization).toBe('Bearer tok');
  });

  it('does not attach a header when there is no token', async () => {
    setTokenGetter(() => null);
    const handlers = (apiClient.interceptors.request as unknown as {
      handlers: Array<{ fulfilled: (c: { headers: Record<string, unknown> }) => unknown }>;
    }).handlers;
    const config = { headers: {} as Record<string, unknown> };
    const result = (await handlers[0].fulfilled(config)) as { headers: Record<string, unknown> };
    expect(result.headers.Authorization).toBeUndefined();
  });
});

describe('downloadObject', () => {
  beforeEach(() => {
    setTokenGetter(() => 'dl-token');
    (globalThis.URL as unknown as { createObjectURL: () => string }).createObjectURL = vi.fn(
      () => 'blob:mock',
    );
    (globalThis.URL as unknown as { revokeObjectURL: () => void }).revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    setTokenGetter(() => null);
    vi.restoreAllMocks();
  });

  it('fetches with the bearer token and triggers an anchor download', async () => {
    const blob = new Blob(['hi'], { type: 'text/plain' });
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      blob: () => Promise.resolve(blob),
    });
    vi.stubGlobal('fetch', fetchMock);
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    await downloadObject('bkt', 'path/to/file name.txt', 'file name.txt');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/buckets/bkt/objects/path/to/file%20name.txt',
      expect.objectContaining({
        method: 'GET',
        headers: { Authorization: 'Bearer dl-token' },
      }),
    );
    expect(clickSpy).toHaveBeenCalled();
  });

  it('throws when the response is not ok', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 403 }));
    await expect(downloadObject('b', 'k', 'k')).rejects.toThrow('403');
  });
});
