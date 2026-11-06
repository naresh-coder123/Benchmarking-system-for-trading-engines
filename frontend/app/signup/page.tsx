'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/context/AuthContext';
import '../globals.css';

export default function SignupPage() {
    const router = useRouter();
    const { signup } = useAuth();
    const [username, setUsername] = useState('');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [teamId, setTeamId] = useState('');
    const [loading, setLoading] = useState(false);
    const [errorMsg, setErrorMsg] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setErrorMsg(null);

        if (password.length < 6) {
            setErrorMsg('Password must be at least 6 characters long.');
            setLoading(false);
            return;
        }

        try {
            await signup(username, email, password, teamId);
            router.push('/submissions');
        } catch (err: any) {
            setErrorMsg(err.message || 'Signup failed. Please try a different username or email.');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-[85vh] flex items-center justify-center p-6">
            <div className="w-full max-w-md bg-[#12151c] border border-[#262d3d] rounded-xl p-8 shadow-2xl">
                <div className="mb-6 text-center">
                    <span className="w-10 h-10 rounded-xl bg-blue-600/20 border border-blue-500/30 inline-flex items-center justify-center text-blue-400 font-mono font-bold text-lg mb-3">
                        ⚡
                    </span>
                    <h1 className="text-xl font-bold tracking-tight text-white">Create Tradebench Account</h1>
                    <p className="text-xs text-[#94a3b8] mt-1">Register to benchmark your trading engines and save history.</p>
                </div>

                {errorMsg && (
                    <div className="mb-5 p-3 rounded-md bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">
                        {errorMsg}
                    </div>
                )}

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label className="block text-xs font-mono uppercase tracking-wider text-[#94a3b8] mb-1.5">
                            Username
                        </label>
                        <input
                            type="text"
                            required
                            autoFocus
                            placeholder="e.g., quant_master"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            className="w-full bg-[#090a0f] border border-[#262d3d] rounded-md px-3.5 py-2.5 text-sm text-white focus:outline-none focus:border-blue-500 font-mono placeholder:text-slate-600"
                        />
                    </div>

                    <div>
                        <label className="block text-xs font-mono uppercase tracking-wider text-[#94a3b8] mb-1.5">
                            Email Address
                        </label>
                        <input
                            type="email"
                            required
                            placeholder="e.g., trader@firm.com"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            className="w-full bg-[#090a0f] border border-[#262d3d] rounded-md px-3.5 py-2.5 text-sm text-white focus:outline-none focus:border-blue-500 font-mono placeholder:text-slate-600"
                        />
                    </div>

                    <div>
                        <label className="block text-xs font-mono uppercase tracking-wider text-[#94a3b8] mb-1.5">
                            Team Identifier (Optional)
                        </label>
                        <input
                            type="text"
                            placeholder="e.g., team-alpha (defaults to username)"
                            value={teamId}
                            onChange={(e) => setTeamId(e.target.value)}
                            className="w-full bg-[#090a0f] border border-[#262d3d] rounded-md px-3.5 py-2.5 text-sm text-white focus:outline-none focus:border-blue-500 font-mono placeholder:text-slate-600"
                        />
                    </div>

                    <div>
                        <label className="block text-xs font-mono uppercase tracking-wider text-[#94a3b8] mb-1.5">
                            Password
                        </label>
                        <input
                            type="password"
                            required
                            placeholder="••••••••••••"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            className="w-full bg-[#090a0f] border border-[#262d3d] rounded-md px-3.5 py-2.5 text-sm text-white focus:outline-none focus:border-blue-500 font-mono placeholder:text-slate-600"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full mt-2 bg-blue-600 hover:bg-blue-500 text-white font-medium py-2.5 px-4 rounded-md text-sm transition-colors duration-150 disabled:opacity-50"
                    >
                        {loading ? 'Registering Account...' : 'Sign Up'}
                    </button>
                </form>

                <div className="mt-6 pt-5 border-t border-[#262d3d] text-center text-xs text-[#94a3b8]">
                    Already have an account?{' '}
                    <Link href="/login" className="text-blue-400 hover:text-blue-300 font-semibold underline underline-offset-2">
                        Sign in here
                    </Link>
                </div>
            </div>
        </div>
    );
}

