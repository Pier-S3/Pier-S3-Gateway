import React from 'react';
import { Breadcrumb as AntBreadcrumb } from 'antd';
import { HomeOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface Props {
  bucket: string;
  prefix: string;
  onNavigate: (prefix: string) => void;
  onHome: () => void;
}

function CrumbLink({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <a
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onClick();
        }
      }}
    >
      {children}
    </a>
  );
}

export default function BreadcrumbNav({ bucket, prefix, onNavigate, onHome }: Props) {
  const { t } = useTranslation();
  const parts = prefix.split('/').filter(Boolean);

  const items = [
    { title: <CrumbLink onClick={onHome}><HomeOutlined /> {t('nav.buckets')}</CrumbLink>, key: 'home' },
    {
      title: <CrumbLink onClick={() => onNavigate('')}>{bucket}</CrumbLink>,
      key: 'bucket',
    },
    ...parts.map((part, i) => {
      const path = parts.slice(0, i + 1).join('/') + '/';
      const isLast = i === parts.length - 1;
      return {
        title: isLast ? part : <CrumbLink onClick={() => onNavigate(path)}>{part}</CrumbLink>,
        key: path,
      };
    }),
  ];

  return <AntBreadcrumb items={items} style={{ marginBottom: 16 }} />;
}
