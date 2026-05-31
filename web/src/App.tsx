import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Spin } from 'antd';
import { AuthProvider, useAuth } from './auth/AuthProvider';
import Login from './pages/Login';
import Callback from './pages/Callback';
import Buckets from './pages/Buckets';
import Browser from './pages/Browser';
import AppShell from './components/AppShell';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  if (isLoading) return <Spin size="large" className="center-spin" />;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <AppShell>{children}</AppShell>;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/callback" element={<Callback />} />
      <Route path="/buckets" element={<ProtectedRoute><Buckets /></ProtectedRoute>} />
      <Route path="/buckets/:bucket" element={<ProtectedRoute><Browser /></ProtectedRoute>} />
      <Route path="*" element={<Navigate to="/buckets" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
