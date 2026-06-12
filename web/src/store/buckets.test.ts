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

  it('merges bucket stats into the matching bucket and skips repeats', async () => {
    mockedGet.mockResolvedValueOnce({
      data: [
        { name: 'b1', permissions: { read: true, write: false } },
        { name: 'b2', permissions: { read: true, write: false } },
      ],
    });
    await useBucketsStore.getState().fetchBuckets();

    mockedGet.mockResolvedValue({
      data: { bucket: 'b1', object_count: 7, size_bytes: 4096, truncated: true, quota_bytes: 8192 },
    });
    await useBucketsStore.getState().fetchBucketStats(['b1']);

    const b1 = useBucketsStore.getState().buckets.find((b) => b.name === 'b1');
    expect(b1).toMatchObject({
      object_count: 7,
      size_bytes: 4096,
      stats_truncated: true,
      quota_bytes: 8192,
    });
    // b2 untouched.
    expect(useBucketsStore.getState().buckets.find((b) => b.name === 'b2')?.size_bytes).toBeUndefined();

    // A second call for the same bucket is a no-op (already requested).
    const calls = mockedGet.mock.calls.length;
    await useBucketsStore.getState().fetchBucketStats(['b1']);
    expect(mockedGet.mock.calls.length).toBe(calls);
  });

  it('leaves the bucket without stats when the stats call fails', async () => {
    mockedGet.mockResolvedValueOnce({
      data: [{ name: 'b1', permissions: { read: true, write: false } }],
    });
    await useBucketsStore.getState().fetchBuckets();

    mockedGet.mockRejectedValue(new Error('403'));
    await useBucketsStore.getState().fetchBucketStats(['b1']);

    const state = useBucketsStore.getState();
    expect(state.buckets[0].size_bytes).toBeUndefined();
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
