import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import * as authApi from '../api/auth'
import { clearToken, setToken as persistToken, setUnauthorizedHandler, getToken } from '../api/client'

interface AuthContextValue {
  username: string | null
  isAuthenticated: boolean
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [username, setUsername] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const logout = () => {
    clearToken()
    setUsername(null)
  }

  useEffect(() => {
    setUnauthorizedHandler(logout)
  }, [])

  // On mount, if a token is already stored (e.g. page refresh), confirm it
  // still works instead of trusting it blindly.
  useEffect(() => {
    if (!getToken()) {
      setLoading(false)
      return
    }
    authApi
      .me()
      .then((res) => setUsername(res.username))
      .catch(() => clearToken())
      .finally(() => setLoading(false))
  }, [])

  const login = async (usernameInput: string, password: string) => {
    const res = await authApi.login(usernameInput, password)
    persistToken(res.token)
    setUsername(res.username)
  }

  const value = useMemo<AuthContextValue>(
    () => ({ username, isAuthenticated: !!username, loading, login, logout }),
    [username, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider')
  return ctx
}
