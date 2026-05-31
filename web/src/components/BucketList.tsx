import { List, Card, Skeleton, Typography, Empty, Tooltip } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { BucketInfo } from '../store/buckets';
import { useNavigate } from 'react-router-dom';

const { Text } = Typography;

interface Props {
  buckets: BucketInfo[];
  loading: boolean;
}

function formatBytes(bytes: number | undefined, dash: string): string {
  if (!bytes) return dash;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  let val = bytes;
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++; }
  return `${val.toFixed(1)} ${units[i]}`;
}

// accessBadge maps a bucket's permissions to a compact label + style class.
// read+write -> R/W (rw), write only -> W (wo), read only -> R (ro).
function accessBadge(
  perms: { read: boolean; write: boolean },
  t: (k: string) => string,
): { label: string; cls: string } {
  if (perms.write && perms.read) return { label: t('buckets.readWrite'), cls: 'rw' };
  if (perms.write) return { label: t('buckets.writeOnly'), cls: 'wo' };
  return { label: t('buckets.read'), cls: 'ro' };
}

export default function BucketList({ buckets, loading }: Props) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const dash = t('common.dash');

  if (loading) return <Skeleton active paragraph={{ rows: 4 }} />;

  if (buckets.length === 0) {
    return <Empty description={t('common.empty')} style={{ marginTop: 48 }} />;
  }

  return (
    <List
      grid={{ gutter: 16, xs: 1, sm: 2, md: 3, lg: 4 }}
      dataSource={buckets}
      renderItem={(bucket) => (
        <List.Item>
          <Card
            hoverable
            className="bucket-card"
            role="button"
            tabIndex={0}
            aria-label={t('common.openFolder', { name: bucket.name })}
            onClick={() => navigate(`/buckets/${bucket.name}`)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                navigate(`/buckets/${bucket.name}`);
              }
            }}
            title={
              <Tooltip title={bucket.name} mouseEnterDelay={0.4}>
                <span className="bucket-title">{bucket.name}</span>
              </Tooltip>
            }
          >
            <div style={{ textAlign: 'center', padding: 'var(--space-3) 0 var(--space-2)' }}>
              <FolderOpenOutlined style={{ fontSize: 44, color: 'var(--dbx-blue)' }} />
            </div>
            <div style={{ textAlign: 'center' }}>
              {bucket.object_count !== undefined && (
                <Text type="secondary">{t('common.objects', { count: bucket.object_count })}</Text>
              )}
              {bucket.size_bytes !== undefined && (
                <Text type="secondary" style={{ marginLeft: 'var(--space-3)' }}>{formatBytes(bucket.size_bytes, dash)}</Text>
              )}
            </div>
            {/* Access is a low-emphasis, isolated footer block - not a loud chip
                next to the name. */}
            <div className="bucket-access">
              <span className="bucket-access-label">{t('buckets.access')}</span>
              {(() => {
                const badge = accessBadge(bucket.permissions, t);
                return (
                  <span className={`bucket-access-value ${badge.cls}`}>{badge.label}</span>
                );
              })()}
            </div>
          </Card>
        </List.Item>
      )}
    />
  );
}
