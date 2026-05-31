import { create } from 'zustand';
import { apiClient, errorMessage } from '../api/client';

export interface ObjectInfo {
  key: string;
  size_bytes: number;
  last_modified: string;
  etag: string;
  is_prefix: boolean;
}

interface ListObjectsResponse {
  objects: ObjectInfo[] | null;
  prefixes: string[] | null;
  next_page_token?: string;
  truncated: boolean;
}

interface BrowserState {
  bucket: string;
  prefix: string;
  objects: ObjectInfo[];
  prefixes: string[];
  loading: boolean;
  error: string | null;
  nextPageToken: string | null;
  truncated: boolean;
  setBucket: (bucket: string) => void;
  setPrefix: (prefix: string) => void;
  fetchObjects: (bucket: string, prefix: string) => Promise<void>;
  loadMore: () => Promise<void>;
}

export const useBrowserStore = create<BrowserState>((set, get) => ({
  bucket: '',
  prefix: '',
  objects: [],
  prefixes: [],
  loading: false,
  error: null,
  nextPageToken: null,
  truncated: false,
  setBucket: (bucket) => set({ bucket, prefix: '', objects: [], prefixes: [] }),
  setPrefix: (prefix) => set({ prefix }),
  fetchObjects: async (bucket, prefix) => {
    // Clear any previously-loaded listing up front so a failed or empty fetch
    // never leaves the previous bucket's objects on screen.
    set({
      loading: true,
      error: null,
      bucket,
      prefix,
      objects: [],
      prefixes: [],
      nextPageToken: null,
      truncated: false,
    });
    try {
      const resp = await apiClient.get<ListObjectsResponse>(
        `/api/v1/buckets/${encodeURIComponent(bucket)}/objects`,
        { params: { prefix, delimiter: '/' } },
      );
      set({
        objects: resp.data.objects ?? [],
        prefixes: resp.data.prefixes ?? [],
        nextPageToken: resp.data.next_page_token ?? null,
        truncated: resp.data.truncated ?? false,
        loading: false,
      });
    } catch (err) {
      // On failure keep the listing cleared (set above) and surface the error.
      set({ error: errorMessage(err, 'Failed to load objects'), loading: false });
    }
  },
  loadMore: async () => {
    const { bucket, prefix, nextPageToken, objects, prefixes } = get();
    if (!nextPageToken) return;
    set({ loading: true });
    try {
      const resp = await apiClient.get<ListObjectsResponse>(
        `/api/v1/buckets/${encodeURIComponent(bucket)}/objects`,
        { params: { prefix, delimiter: '/', page_token: nextPageToken } },
      );
      set({
        objects: [...objects, ...(resp.data.objects ?? [])],
        prefixes: [...prefixes, ...(resp.data.prefixes ?? [])],
        nextPageToken: resp.data.next_page_token ?? null,
        truncated: resp.data.truncated ?? false,
        loading: false,
      });
    } catch (err) {
      set({ error: errorMessage(err, 'Failed to load more'), loading: false });
    }
  },
}));
