import React, { useEffect } from 'react';
import { Layout, Menu, Avatar, Dropdown, Typography, Divider, theme } from 'antd';
import {
  FolderOutlined,
  DatabaseOutlined,
  LogoutOutlined,
  DownOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../auth/AuthProvider';
import { useBucketsStore } from '../store/buckets';
import Logo from './Logo';
import LanguageSwitcher from './LanguageSwitcher';
import ThemeSwitcher from '../theme/ThemeSwitcher';

const { Sider, Header, Content } = Layout;
const { Text } = Typography;

export default function AppShell({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { bucket } = useParams<{ bucket: string }>();
  const { user, logout } = useAuth();
  const { buckets, fetchBuckets } = useBucketsStore();
  const { token } = theme.useToken();

  useEffect(() => {
    if (buckets.length === 0) fetchBuckets();
  }, [buckets.length, fetchBuckets]);

  const headerTitle = bucket ?? t('buckets.title');

  const selectedKey = bucket ? `bucket:${bucket}` : 'all';

  const menuItems = [
    {
      key: 'all',
      icon: <DatabaseOutlined />,
      label: t('nav.allBuckets'),
    },
    ...buckets.map((b) => ({
      key: `bucket:${b.name}`,
      icon: <FolderOutlined />,
      label: b.name,
    })),
  ];

  const onMenuClick = ({ key }: { key: string }) => {
    if (key === 'all') {
      navigate('/buckets');
    } else if (key.startsWith('bucket:')) {
      navigate(`/buckets/${key.slice('bucket:'.length)}`);
    }
  };

  const initial = user?.username?.charAt(0).toUpperCase() ?? '?';

  const userMenu = {
    items: [
      {
        key: 'logout',
        icon: <LogoutOutlined />,
        label: t('auth.logout'),
        onClick: () => { void logout(); },
      },
    ],
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider className="app-sider" width={240} breakpoint="lg" collapsedWidth={0}>
        <div className="app-logo">
          <Logo size={24} />
          <span>{t('common.appName')}</span>
        </div>
        <div className="sider-nav-label">{t('nav.buckets')}</div>
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={menuItems}
          onClick={onMenuClick}
          style={{ borderInlineEnd: 'none' }}
        />
      </Sider>
      <Layout>
        <Header className="app-header">
          <div className="app-header-title">{headerTitle}</div>
          <div className="app-header-right">
            <ThemeSwitcher />
            <Divider type="vertical" style={{ margin: 0 }} />
            <LanguageSwitcher />
            {user && (
              <>
                <Divider type="vertical" style={{ margin: 0 }} />
                <Dropdown menu={userMenu} trigger={['click']} placement="bottomRight">
                  <button
                    type="button"
                    className="user-menu-trigger"
                    aria-label={t('auth.logout')}
                  >
                    <Avatar
                      size={28}
                      style={{ backgroundColor: token.colorPrimary }}
                    >
                      {initial}
                    </Avatar>
                    <Text className="user-menu-name">{user.username}</Text>
                    <DownOutlined className="user-menu-caret" />
                  </button>
                </Dropdown>
              </>
            )}
          </div>
        </Header>
        <Content className="app-content">{children}</Content>
      </Layout>
    </Layout>
  );
}
