import React, { createContext, useContext, useEffect, useState, useCallback, useRef } from 'react';
import { UserManager, WebStorageStateStore, User } from 'oidc-client-ts';
import { apiClient, setTokenGetter, errorMessage } from '../api/client';

interface UserInfo {
  username: string;
  groups: string[];
}

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: UserInfo | null;
  login: () => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType>({
  isAuthenticated: false,
  isLoading: true,
  user: null,
  login: () => {},
  logout: () => {},
});

export function useAuth(): AuthContextType {
  return useContext(AuthContext);
}

// createUserManager builds the OIDC UserManager. It is created lazily (inside
// the provider, not at module-evaluation time) so that importing this module
// does not touch `window`/`import.meta.env` before the DOM is ready, which
// would throw in SSR/test environments and would also wire up the library's
// silent-renew timers at import time.
function createUserManager(): UserManager {
  return new UserManager({
    authority: import.meta.env.VITE_KEYCLOAK_URL || 'https://keycloak.example.com/realms/myrealm',
    client_id: import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 's3-proxy',
    // Dedicated callback route keeps the OIDC redirect handler separate from the
    // login page and avoids brittle `code=` query sniffing on /login.
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: window.location.origin,
    response_type: 'code',
    scope: 'openid profile',
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  });
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  // The raw access token is intentionally NOT stored in React state: keeping it
  // out of the component tree avoids a full subtree re-render on every silent
  // renewal and keeps the secret out of React DevTools / error-boundary dumps.
  // HTTP clients read it on demand via the ref below.
  const tokenRef = useRef<string | null>(null);
  // Lazily construct a single UserManager for the lifetime of the provider.
  const userManagerRef = useRef<UserManager | null>(null);
  if (userManagerRef.current === null) {
    userManagerRef.current = createUserManager();
  }
  const userManager = userManagerRef.current;

  // Wire the axios client to read the current token. Done once, synchronously,
  // so the very first authenticated request carries the Bearer header.
  useEffect(() => {
    setTokenGetter(() => tokenRef.current);
  }, []);

  const handleUser = useCallback(async (oidcUser: User | null) => {
    if (oidcUser && !oidcUser.expired) {
      tokenRef.current = oidcUser.access_token;
      try {
        const resp = await apiClient.get<UserInfo>('/api/v1/auth/me');
        setUser(resp.data);
      } catch (err) {
        // Token is valid for OIDC but /me failed; fall back to profile claims.
        void errorMessage(err, '');
        setUser({
          username: oidcUser.profile.preferred_username || 'unknown',
          groups: [],
        });
      }
    } else {
      tokenRef.current = null;
      setUser(null);
    }
    setIsLoading(false);
  }, []);

  useEffect(() => {
    let active = true;
    const onUser = (u: User | null) => {
      if (active) void handleUser(u);
    };

    if (window.location.pathname === '/callback') {
      userManager
        .signinRedirectCallback()
        .then((u) => onUser(u))
        .catch(() => active && setIsLoading(false));
    } else {
      userManager
        .getUser()
        .then((u) => onUser(u))
        .catch(() => active && setIsLoading(false));
    }

    // Hold identity references so StrictMode's double-invocation in dev does
    // not leave duplicate handlers registered. handleLoaded is an alias of
    // onUser (User widens to User | null); the others adapt zero-arg events.
    //
    // automaticSilentRenew: true already drives renewal and fires userLoaded
    // (handleLoaded). We therefore do NOT call signinSilent() manually - it
    // would race the automatic renewal and could issue two parallel
    // token-endpoint requests. We only react when the library reports silent
    // renewal is impossible, which is the correct trigger for a forced
    // sign-out.
    const handleLoaded: (u: User) => void = onUser;
    const handleUnloaded = () => onUser(null);
    const handleSilentRenewError = () => onUser(null);

    userManager.events.addUserLoaded(handleLoaded);
    userManager.events.addUserUnloaded(handleUnloaded);
    userManager.events.addSilentRenewError(handleSilentRenewError);

    return () => {
      active = false;
      userManager.events.removeUserLoaded(handleLoaded);
      userManager.events.removeUserUnloaded(handleUnloaded);
      userManager.events.removeSilentRenewError(handleSilentRenewError);
    };
  }, [handleUser, userManager]);

  const login = useCallback(() => {
    void userManager.signinRedirect();
  }, [userManager]);

  const logout = useCallback(async () => {
    try {
      await apiClient.post('/api/v1/auth/logout');
    } catch {
      /* ignore logout endpoint failures; proceed to OIDC sign-out anyway */
    }
    await userManager.signoutRedirect();
  }, [userManager]);

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: !!user,
        isLoading,
        user,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
