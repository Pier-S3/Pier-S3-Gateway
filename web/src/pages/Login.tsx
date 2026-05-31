import { useEffect } from "react";
import { useAuth } from '../auth/AuthProvider';
import { useNavigate } from 'react-router-dom';
import { Card, Button, Spin, Typography, Divider } from 'antd';
import { LoginOutlined } from '@ant-design/icons';
import Logo from '../components/Logo';
import { useTranslation } from 'react-i18next';
import LanguageSwitcher from '../components/LanguageSwitcher';
import ThemeSwitcher from '../theme/ThemeSwitcher';

const { Title, Paragraph } = Typography;

export default function Login() {
  const { t } = useTranslation();
  const { isAuthenticated, isLoading, login } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (isAuthenticated) navigate('/buckets', { replace: true });
  }, [isAuthenticated, navigate]);

  if (isLoading) return <Spin size="large" className="center-spin" />;

  return (
    <div className="login-wrap">
      <Card className="login-card">
        <div className="login-toolbar">
          <ThemeSwitcher />
          <Divider type="vertical" style={{ margin: 0 }} />
          <LanguageSwitcher />
        </div>
        <div className="login-logo" style={{ marginBottom: 8 }}>
          <Logo size={48} />
        </div>
        <Title level={3} style={{ marginTop: 8 }}>{t('auth.title')}</Title>
        <Paragraph type="secondary">{t('auth.subtitle')}</Paragraph>
        <Button
          type="primary"
          icon={<LoginOutlined />}
          size="large"
          block
          onClick={login}
        >
          {t('auth.signIn')}
        </Button>
      </Card>
    </div>
  );
}
