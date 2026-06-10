import { useEffect } from "react";
import { Alert } from 'antd';
import { useTranslation } from 'react-i18next';
import { useBucketsStore } from '../store/buckets';
import BucketList from '../components/BucketList';

export default function Buckets() {
  const { t } = useTranslation();
  const { buckets, loading, error, fetchBuckets } = useBucketsStore();

  useEffect(() => { fetchBuckets(); }, [fetchBuckets]);

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
