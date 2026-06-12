import { create } from 'zustand';
import { apiClient, errorMessage } from '../api/client';

export interface BucketInfo {
  name: string;
  permissions: { read: boolean; write: boolean; delete?: boolean };
  object_count?: number;
  size_bytes?: number;
  /** True when the server-side stats walk hit its cap: totals are a lower bound. */
  stats_truncated?: boolean;
  /** Configured quota in bytes (0/undefined = no quota). */
  quota_bytes?: number;
}

interface BucketStats {
  bucket: string;
  object_count: number;
  size_bytes: number;
  truncated: boolean;
  quota_bytes?: number;
}

interface BucketsState {
  buckets: BucketInfo[];
  loading: boolean;
  error: string | null;
  fetchBuckets: () => Promise<void>;
  /**
   * Fetch size/quota stats for the given buckets and merge them into the
   * list as each response arrives. Requests run with a small concurrency
   * cap so a long bucket list does not burst the gateway; failures are
   * silent (the card simply shows no stats).
   */
  fetchBucketStats: (names: string[]) => Promise<void>;
}

/** How many stats requests may be in flight at once. */
const STATS_CONCURRENCY = 4;

// Names already requested in this listing session. The Buckets page effect
// re-fires every time stats merge into the list; without this guard each
// merge would re-request the buckets still in flight.
const statsRequested = new Set<string>();

export const useBucketsStore = create<BucketsState>((set) => ({
  buckets: [],
  loading: false,
  error: null,
  fetchBuckets: async () => {
    set({ loading: true, error: null });
    try {
      const resp = await apiClient.get<BucketInfo[]>('/api/v1/buckets');
      statsRequested.clear();
      set({ buckets: resp.data, loading: false });
    } catch (err) {
      set({ error: errorMessage(err, 'Failed to load buckets'), loading: false });
    }
  },
  fetchBucketStats: async (names: string[]) => {
    const queue = names.filter((n) => !statsRequested.has(n));
    queue.forEach((n) => statsRequested.add(n));
    const worker = async () => {
      for (let name = queue.shift(); name !== undefined; name = queue.shift()) {
        try {
          const resp = await apiClient.get<BucketStats>(
            `/api/v1/buckets/${encodeURIComponent(name)}/stats`,
          );
          const stats = resp.data;
          set((state) => ({
            buckets: state.buckets.map((b) =>
              b.name === name
                ? {
                    ...b,
                    object_count: stats.object_count,
                    size_bytes: stats.size_bytes,
                    stats_truncated: stats.truncated,
                    quota_bytes: stats.quota_bytes,
                  }
                : b,
            ),
          }));
        } catch {
          // Stats are decorative: a failed/forbidden lookup leaves the card
          // without numbers rather than surfacing an error banner.
        }
      }
    };
    await Promise.all(
      Array.from({ length: Math.min(STATS_CONCURRENCY, queue.length) }, () => worker()),
    );
  },
}));
