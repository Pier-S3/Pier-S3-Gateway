import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import BucketList from './BucketList';
import { BucketInfo } from '../store/buckets';

const navigateMock = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigateMock };
});

const buckets: BucketInfo[] = [
  { name: 'read-only', permissions: { read: true, write: false } },
  { name: 'read-write', permissions: { read: true, write: true } },
];

function renderList(loading = false) {
  return render(
    <MemoryRouter>
      <BucketList buckets={buckets} loading={loading} />
    </MemoryRouter>,
  );
}

describe('BucketList', () => {
  it('shows a skeleton while loading', () => {
    const { container } = renderList(true);
    expect(container.querySelector('.ant-skeleton')).toBeTruthy();
  });

  it('renders one card per bucket with permission tags', () => {
    renderList();
    expect(screen.getByText('read-only')).toBeInTheDocument();
    expect(screen.getByText('read-write')).toBeInTheDocument();
    expect(screen.getByText('R/W')).toBeInTheDocument();
    expect(screen.getByText('R')).toBeInTheDocument();
  });

  it('navigates to the bucket browser on click', () => {
    navigateMock.mockReset();
    renderList();
    fireEvent.click(screen.getByText('read-only'));
    expect(navigateMock).toHaveBeenCalledWith('/buckets/read-only');
  });
});
