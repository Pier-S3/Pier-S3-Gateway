import { useEffect, useMemo } from "react";
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { Alert, Button } from 'antd';
import { useTranslation } from 'react-i18next';
import { useBrowserStore } from '../store/browser';
import { useBucketsStore } from '../store/buckets';
import BreadcrumbNav from '../components/Breadcrumb';
import ObjectBrowser from '../components/ObjectBrowser';
import UploadZone from '../components/UploadZone';

export default function Browser() {
  const { t } = useTranslation();
  const { bucket } = useParams<{ bucket: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  // The current folder prefix is driven by the URL (?prefix=...), NOT by
  // persisted store state. Entering a bucket always lands on /buckets/<bucket>
  // with no ?prefix (every entry point - bucket card, sidebar, breadcrumb home -
  // navigates to the bare bucket URL), so switching buckets reliably starts at
  // the new bucket's root instead of inheriting the previous bucket's folder.
  // It also makes browser back/forward and deep links work.
  const prefix = searchParams.get('prefix') ?? '';
  const { objects, prefixes, loading, error, truncated, fetchObjects, loadMore } = useBrowserStore();
  const { buckets, fetchBuckets } = useBucketsStore();

  useEffect(() => {
    if (buckets.length === 0) fetchBuckets();
  }, [buckets.length, fetchBuckets]);

  const bucketInfo = useMemo(() => buckets.find((b) => b.name === bucket), [buckets, bucket]);
  const canWrite = bucketInfo?.permissions.write ?? false;
  // Read is assumed until we know the bucket (so a deep-link still tries to
  // list); once we know it's write-only, skip the listing rather than firing a
  // request that will 403 and surface a scary error.
  const canRead = bucketInfo ? bucketInfo.permissions.read : true;

  useEffect(() => {
    if (bucket && canRead) fetchObjects(bucket, prefix);
  }, [bucket, prefix, canRead, fetchObjects]);

  const handleNavigatePrefix = (newPrefix: string) => {
    // Drive folder navigation through the URL so it composes with router history
    // and never lingers across a bucket switch.
    setSearchParams(newPrefix ? { prefix: newPrefix } : {});
  };

  const handleRefresh = () => {
    if (bucket) fetchObjects(bucket, prefix);
  };

  if (!bucket) return null;

  return (
    <div>
      <BreadcrumbNav
        bucket={bucket}
        prefix={prefix}
        onNavigate={handleNavigatePrefix}
        onHome={() => navigate('/buckets')}
      />
      {/* Write-only bucket: listing is intentionally not allowed, so explain
          that calmly (info, not error) instead of showing a failed request. */}
      {!canRead && (
        <Alert
          message={t('browser.writeOnlyTitle')}
          description={t('browser.writeOnlyHint')}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
      {canRead && error && (
        <Alert
          message={t('errors.loadObjects')}
          description={error}
          type="error"
          showIcon
          action={<Button size="small" type="link" onClick={handleRefresh}>{t('common.retry')}</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      {canWrite && <UploadZone bucket={bucket} prefix={prefix} onUploaded={handleRefresh} />}
      {canRead && (
        <ObjectBrowser
          bucket={bucket}
          prefix={prefix}
          objects={objects}
          prefixes={prefixes}
          loading={loading}
          canWrite={canWrite}
          truncated={truncated}
          onNavigatePrefix={handleNavigatePrefix}
          onLoadMore={loadMore}
          onRefresh={handleRefresh}
        />
      )}
    </div>
  );
}
