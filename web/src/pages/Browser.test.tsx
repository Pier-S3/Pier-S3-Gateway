import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';

// Mock the API client; use the REAL browser store so the regression (a prefix
// persisting in store state across a bucket switch) is actually exercised.
vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return { ...actual, apiClient: { get: vi.fn() } };
});

// Stable buckets-store value: both buckets are readable so the listing fires.
const bucketsStoreValue = {
  buckets: [
    { name: 'alpha', permissions: { read: true, write: false } },
    { name: 'beta', permissions: { read: true, write: false } },
  ],
  fetchBuckets: vi.fn(),
};
vi.mock('../store/buckets', () => ({
  useBucketsStore: () => bucketsStoreValue,
}));

import { apiClient } from '../api/client';
import { useBrowserStore } from '../store/browser';
import Browser from './Browser';

const mockedGet = apiClient.get as unknown as ReturnType<typeof vi.fn>;

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/buckets/:bucket" element={<Browser />} />
      </Routes>
    </MemoryRouter>,
  );
}

const listParams = (bucket: string, prefix: string) => [
  `/api/v1/buckets/${bucket}/objects`,
  { params: { prefix, delimiter: '/' } },
];

describe('Browser - prefix is URL-driven', () => {
  beforeEach(() => {
    mockedGet.mockReset();
    mockedGet.mockResolvedValue({ data: { objects: [], prefixes: [], truncated: false } });
    useBrowserStore.setState({ bucket: '', prefix: '', objects: [], prefixes: [], nextPageToken: null, truncated: false });
  });

  it('lists the bucket at the prefix taken from ?prefix', async () => {
    renderAt('/buckets/alpha?prefix=css/');
    await waitFor(() => expect(mockedGet).toHaveBeenCalledWith(...listParams('alpha', 'css/')));
  });

  it('switching to another bucket does NOT inherit the previous prefix', async () => {
    // Browse into alpha/css/ - this leaves prefix="css/" in the shared store.
    renderAt('/buckets/alpha?prefix=css/');
    await waitFor(() => expect(mockedGet).toHaveBeenCalledWith(...listParams('alpha', 'css/')));

    cleanup();
    mockedGet.mockClear();

    // Enter beta via its bare URL (how every bucket entry point navigates).
    renderAt('/buckets/beta');
    await waitFor(() => expect(mockedGet).toHaveBeenCalledWith(...listParams('beta', '')));
    // Must never list beta under alpha's leftover prefix.
    expect(mockedGet).not.toHaveBeenCalledWith(...listParams('beta', 'css/'));
  });
});
