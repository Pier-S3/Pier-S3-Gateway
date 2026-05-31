import { useEffect } from "react";
import { Typography, Alert } from 'antd';
import { useTranslation } from 'react-i18next';
import { useBucketsStore } from '../store/buckets';
import BucketList from '../components/BucketList';

const { Title } = Typography;

export default function Buckets() {
  const { t } = useTranslation();
  const { buckets, loading, error, fetchBuckets } = useBucketsStore();

  useEffect(() => { fetchBuckets(); }, [fetchBuckets]);

  return (
    <div>
      <Title level={3} style={{ marginTop: 0 }}>{t('buckets.title')}</Title>
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
