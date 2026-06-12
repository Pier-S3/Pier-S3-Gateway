import { useEffect } from "react";
import { Alert } from 'antd';
import { useTranslation } from 'react-i18next';
import { useBucketsStore } from '../store/buckets';
import BucketList from '../components/BucketList';

export default function Buckets() {
  const { t } = useTranslation();
  const { buckets, loading, error, fetchBuckets, fetchBucketStats } = useBucketsStore();

  useEffect(() => { fetchBuckets(); }, [fetchBuckets]);

  // Decorate the cards with size/quota once the list is known. Only readable
  // buckets are asked: the stats endpoint mirrors listing ACL, so a
  // write-only bucket would just 403.
  useEffect(() => {
    const readable = buckets.filter((b) => b.permissions.read && b.size_bytes === undefined);
    if (readable.length > 0) void fetchBucketStats(readable.map((b) => b.name));
  }, [buckets, fetchBucketStats]);

  return (
    <div>
      <h1 className="page-title">{t('buckets.title')}</h1>
      {error && (
        <Alert
          message={t('errors.loadBuckets')}
          description={error}
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
      <BucketList buckets={buckets} loading={loading} />
    </div>
  );
}
