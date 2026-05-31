import { useEffect } from "react";
import { Navigate } from 'react-router-dom';
import { Spin } from 'antd';
import { useAuth } from '../auth/AuthProvider';

/**
 * OIDC redirect target. The AuthProvider effect processes the authorization
 * code (signinRedirectCallback) when the path is /callback; this page simply
 * shows a spinner until auth state resolves, then routes onward.
 */
export default function Callback() {
  const { isLoading, isAuthenticated } = useAuth();

  // Clear the `?code=&state=` query from the URL once processing is underway so
  // a reload does not attempt to redeem a stale code.
  useEffect(() => {
    if (!isLoading && window.location.search) {
      window.history.replaceState({}, document.title, '/callback');
    }
  }, [isLoading]);

  if (isLoading) {
    return <Spin size="large" style={{ display: 'flex', justifyContent: 'center', marginTop: 200 }} />;
  }

  return <Navigate to={isAuthenticated ? '/buckets' : '/login'} replace />;
}
