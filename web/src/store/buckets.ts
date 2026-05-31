import { create } from 'zustand';
import { apiClient, errorMessage } from '../api/client';

export interface BucketInfo {
  name: string;
  permissions: { read: boolean; write: boolean; delete?: boolean };
  object_count?: number;
  size_bytes?: number;
}

interface BucketsState {
  buckets: BucketInfo[];
  loading: boolean;
  error: string | null;
  fetchBuckets: () => Promise<void>;
}

export const useBucketsStore = create<BucketsState>((set) => ({
  buckets: [],
  loading: false,
  error: null,
  fetchBuckets: async () => {
    set({ loading: true, error: null });
    try {
      const resp = await apiClient.get<BucketInfo[]>('/api/v1/buckets');
      set({ buckets: resp.data, loading: false });
    } catch (err) {
      set({ error: errorMessage(err, 'Failed to load buckets'), loading: false });
    }
  },
}));
