import { List, Card, Skeleton, Empty, Tooltip } from 'antd';
import { FolderOpenOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { BucketInfo } from '../store/buckets';
import { useNavigate } from 'react-router-dom';

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
      grid={{ gutter: 16, xs: 1, sm: 2, md: 2, lg: 3, xl: 3, xxl: 4 }}
      dataSource={buckets}
      renderItem={(bucket) => {
        const badge = accessBadge(bucket.permissions, t);
        const hasStats =
          bucket.object_count !== undefined || bucket.size_bytes !== undefined;
        return (
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
            >
              {/* Compact row-card: icon anchor, name (grows), access pill - one
                  baseline, no dead space, and stats slot in as a second line
                  when a backend supplies object_count / size_bytes. */}
              <span className="bucket-icon-tile">
                <FolderOpenOutlined />
              </span>
              <div className="bucket-card-info">
                <Tooltip title={bucket.name} mouseEnterDelay={0.4}>
                  <span className="bucket-card-name">{bucket.name}</span>
                </Tooltip>
                {hasStats && (
                  <div className="bucket-stats">
                    {bucket.object_count !== undefined && (
                      <span>{t('common.objects', { count: bucket.object_count })}</span>
                    )}
                    {bucket.size_bytes !== undefined && (
                      <span>{formatBytes(bucket.size_bytes, dash)}</span>
                    )}
                  </div>
                )}
              </div>
              <span
                className={`bucket-access-value ${badge.cls}`}
                title={`${t('buckets.access')}: ${badge.label}`}
              >
                {badge.label}
              </span>
            </Card>
          </List.Item>
        );
      }}
    />
  );
}
