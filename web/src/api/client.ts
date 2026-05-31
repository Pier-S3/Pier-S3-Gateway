import axios, { AxiosError } from 'axios';

export const apiClient = axios.create({
  baseURL: '',
  headers: { 'Content-Type': 'application/json' },
});

// The access-token getter is wired up by the AuthProvider via setTokenGetter.
// Until then it returns null and requests go out unauthenticated.
let getAccessToken: () => string | null = () => null;

export function setTokenGetter(getter: () => string | null): void {
  getAccessToken = getter;
}

export function getToken(): string | null {
  return getAccessToken();
}

apiClient.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token && !config.headers.Authorization) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      // Only redirect when we are not already on an auth route, to avoid loops.
      const path = window.location.pathname;
      if (path !== '/login' && path !== '/callback') {
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  },
);

/**
 * Build the path for an object endpoint matching the backend `{key...}` wildcard
 * route. Each path segment is percent-encoded individually so that special
 * characters (space, '#', '?', '%', unicode) are escaped, while the '/'
 * separators that express the object's pseudo-directory structure are preserved
 * literally (the Go mux matches them as path separators, not %2F).
 */
function encodeKey(key: string): string {
  return key
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

export function buildObjectPath(bucket: string, key: string): string {
  return `/api/v1/buckets/${encodeURIComponent(bucket)}/objects/${encodeKey(key)}`;
}

/**
 * Build the object-metadata path. The backend exposes metadata as a PREFIX
 * route (`/buckets/{bucket}/meta/{key...}`) rather than `/objects/{key}/meta`,
 * because Go 1.22's mux cannot place a literal segment after a `{key...}`
 * wildcard. Requesting `/objects/{key}/meta` would instead match the
 * get-object route with key `"{key}/meta"` and 404.
 */
export function buildMetaPath(bucket: string, key: string): string {
  return `/api/v1/buckets/${encodeURIComponent(bucket)}/meta/${encodeKey(key)}`;
}

/**
 * Extract a human-friendly error message from an axios error, falling back to a
 * provided default. The backend returns `{ error, message }` envelopes.
 */
export function errorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { message?: string } | undefined;
    if (data?.message) {
      return data.message;
    }
  }
  return fallback;
}

/**
 * Download an object using an authenticated fetch. A native anchor navigation
 * cannot attach the Bearer header, so we fetch the bytes with the token, turn
 * them into an object URL, and trigger a download from that.
 */
export async function fetchObjectBlob(bucket: string, key: string): Promise<Blob> {
  const token = getAccessToken();
  const headers: Record<string, string> = {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(buildObjectPath(bucket, key), {
    method: 'GET',
    headers,
  });

  if (!response.ok) {
    throw new Error(`request failed with status ${response.status}`);
  }

  return response.blob();
}

export async function downloadObject(bucket: string, key: string, fileName: string): Promise<void> {
  const blob = await fetchObjectBlob(bucket, key);
  const objectUrl = URL.createObjectURL(blob);
  try {
    const anchor = document.createElement('a');
    anchor.href = objectUrl;
    anchor.download = fileName;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
  } finally {
    URL.revokeObjectURL(objectUrl);
  }
}
