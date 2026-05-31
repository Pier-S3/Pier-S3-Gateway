import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    apiClient: { get: vi.fn() },
  };
});

import { apiClient } from '../api/client';
import { useBrowserStore } from './browser';

const mockedGet = apiClient.get as unknown as ReturnType<typeof vi.fn>;

describe('useBrowserStore', () => {
  beforeEach(() => {
    mockedGet.mockReset();
    useBrowserStore.setState({
      bucket: '',
      prefix: '',
      objects: [],
      prefixes: [],
      loading: false,
      error: null,
      nextPageToken: null,
      truncated: false,
    });
  });

  it('fetchObjects requests with prefix and delimiter and stores results', async () => {
    mockedGet.mockResolvedValue({
      data: {
        objects: [{ key: 'a.txt', size_bytes: 10, last_modified: '', etag: '', is_prefix: false }],
        prefixes: ['sub/'],
        next_page_token: 'tok',
        truncated: true,
      },
    });

    await useBrowserStore.getState().fetchObjects('mybucket', 'sub/');

    expect(mockedGet).toHaveBeenCalledWith(
      '/api/v1/buckets/mybucket/objects',
      { params: { prefix: 'sub/', delimiter: '/' } },
    );
    const state = useBrowserStore.getState();
    expect(state.objects).toHaveLength(1);
    expect(state.prefixes).toEqual(['sub/']);
    expect(state.nextPageToken).toBe('tok');
    expect(state.truncated).toBe(true);
  });

  it('handles null objects/prefixes from the backend', async () => {
    mockedGet.mockResolvedValue({
      data: { objects: null, prefixes: null, truncated: false },
    });
    await useBrowserStore.getState().fetchObjects('b', '');
    const state = useBrowserStore.getState();
    expect(state.objects).toEqual([]);
    expect(state.prefixes).toEqual([]);
    expect(state.nextPageToken).toBeNull();
  });

  it('loadMore appends results using the page token', async () => {
    useBrowserStore.setState({
      bucket: 'b',
      prefix: '',
      objects: [{ key: 'first.txt', size_bytes: 1, last_modified: '', etag: '', is_prefix: false }],
      prefixes: [],
      nextPageToken: 'page2',
      truncated: true,
    });
    mockedGet.mockResolvedValue({
      data: {
        objects: [{ key: 'second.txt', size_bytes: 2, last_modified: '', etag: '', is_prefix: false }],
        prefixes: [],
        truncated: false,
      },
    });

    await useBrowserStore.getState().loadMore();

    expect(mockedGet).toHaveBeenCalledWith(
      '/api/v1/buckets/b/objects',
      { params: { prefix: '', delimiter: '/', page_token: 'page2' } },
    );
    const state = useBrowserStore.getState();
    expect(state.objects.map((o) => o.key)).toEqual(['first.txt', 'second.txt']);
    expect(state.truncated).toBe(false);
  });

  it('loadMore is a no-op without a page token', async () => {
    useBrowserStore.setState({ nextPageToken: null });
    await useBrowserStore.getState().loadMore();
    expect(mockedGet).not.toHaveBeenCalled();
  });

  it('setBucket resets prefix and lists', () => {
    useBrowserStore.setState({ prefix: 'x/', objects: [{ key: 'k', size_bytes: 0, last_modified: '', etag: '', is_prefix: false }] });
    useBrowserStore.getState().setBucket('newb');
    const state = useBrowserStore.getState();
    expect(state.bucket).toBe('newb');
    expect(state.prefix).toBe('');
    expect(state.objects).toEqual([]);
  });
});
