'use client';

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';

export interface User {
    id: number;
    username: string;
    email: string;
    team_id: string;
    created_at: string;
}

interface AuthContextType {
    user: User | null;
    token: string | null;
    isLoading: boolean;
    isAuthenticated: boolean;
    login: (usernameOrEmail: string, password: string) => Promise<void>;
    signup: (username: string, email: string, password: string, teamId?: string) => Promise<void>;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_BASE = 'http://localhost:8080';
const TOKEN_KEY = 'tradebench_jwt_token';

export function AuthProvider({ children }: { children: ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [token, setToken] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    // Rehydrate session from localStorage on mount
    useEffect(() => {
        const storedToken = localStorage.getItem(TOKEN_KEY);
        if (!storedToken) {
            setIsLoading(false);
            return;
        }

        setToken(storedToken);

        fetch(`${API_BASE}/api/auth/me`, {
            headers: {
                Authorization: `Bearer ${storedToken}`,
            },
        })
            .then(async (res) => {
                if (res.ok) {
                    const data = await res.json();
                    setUser(data.user);
                } else {
                    // Invalid or expired token
                    localStorage.removeItem(TOKEN_KEY);
                    setToken(null);
                    setUser(null);
                }
            })
            .catch(() => {
                // If API is unreachable, keep token in storage or clear based on preference
                console.warn('API unreachable during session rehydration');
            })
            .finally(() => {
                setIsLoading(false);
            });
    }, []);

    const login = async (username: string, password: string) => {
        const res = await fetch(`${API_BASE}/api/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'Login failed');
        }

        const data = await res.json();
        localStorage.setItem(TOKEN_KEY, data.token);
        setToken(data.token);
        setUser(data.user);
    };

    const signup = async (username: string, email: string, password: string, teamId?: string) => {
        const res = await fetch(`${API_BASE}/api/auth/signup`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                username,
                email,
                password,
                team_id: teamId || username,
            }),
        });

        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || 'Signup failed');
        }

        const data = await res.json();
        localStorage.setItem(TOKEN_KEY, data.token);
        setToken(data.token);
        setUser(data.user);
    };

    const logout = () => {
        localStorage.removeItem(TOKEN_KEY);
        setToken(null);
        setUser(null);
    };

    return (
        <AuthContext.Provider
            value={{
                user,
                token,
                isLoading,
                isAuthenticated: !!user,
                login,
                signup,
                logout,
            }}
        >
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (!context) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
}

