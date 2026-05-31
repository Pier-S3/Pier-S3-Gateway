import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosError, AxiosHeaders } from 'axios';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    apiClient: { get: vi.fn() },
  };
});

import { apiClient } from '../api/client';
import { useBucketsStore } from './buckets';

const mockedGet = apiClient.get as unknown as ReturnType<typeof vi.fn>;

describe('useBucketsStore', () => {
  beforeEach(() => {
    mockedGet.mockReset();
    useBucketsStore.setState({ buckets: [], loading: false, error: null });
  });

  it('loads buckets on success', async () => {
    mockedGet.mockResolvedValue({
      data: [{ name: 'b1', permissions: { read: true, write: false } }],
    });
    await useBucketsStore.getState().fetchBuckets();
    const state = useBucketsStore.getState();
    expect(state.buckets).toHaveLength(1);
    expect(state.buckets[0].name).toBe('b1');
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it('records the backend error message on failure', async () => {
    const headers = new AxiosHeaders();
    mockedGet.mockRejectedValue(
      new AxiosError('x', 'E', undefined, undefined, {
        status: 502,
        data: { message: 'upstream down' },
        statusText: '',
        headers,
        config: { headers },
      }),
    );
    await useBucketsStore.getState().fetchBuckets();
    const state = useBucketsStore.getState();
    expect(state.error).toBe('upstream down');
    expect(state.loading).toBe(false);
  });
});
