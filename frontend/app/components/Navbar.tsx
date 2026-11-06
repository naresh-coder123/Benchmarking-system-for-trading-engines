'use client';

import React from 'react';
import Link from 'next/link';
import { useAuth } from '@/context/AuthContext';

export default function Navbar() {
    const { user, isAuthenticated, logout } = useAuth();

    return (
        <nav className="border-b border-[#262d3d] bg-[#0d0f14]/90 backdrop-blur sticky top-0 z-50">
            <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
                <Link href="/leaderboard" className="flex items-center gap-2.5 group">
                    <span className="w-8 h-8 rounded-lg bg-blue-600/20 border border-blue-500/30 flex items-center justify-center text-blue-400 font-mono font-bold text-sm group-hover:bg-blue-600/30 transition-colors">
                        ⚡
                    </span>
                    <div>
                        <span className="font-bold text-sm tracking-tight text-white block">
                            Tradebench
                        </span>
                        <span className="text-[10px] font-mono text-[#94a3b8] block -mt-0.5">
                            Matching Engine Telemetry
                        </span>
                    </div>
                </Link>

                <div className="flex items-center gap-1 sm:gap-2 font-mono text-xs">
                    <Link
                        href="/submit"
                        className="px-3.5 py-1.5 rounded-md hover:bg-[#181c25] text-slate-300 hover:text-white transition-colors"
                    >
                        Submit Engine
                    </Link>
                    <Link
                        href="/monitor"
                        className="px-3.5 py-1.5 rounded-md hover:bg-[#181c25] text-slate-300 hover:text-white transition-colors flex items-center gap-1.5"
                    >
                        <span className="w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse" />
                        Live Monitor
                    </Link>
                    <Link
                        href="/leaderboard"
                        className="px-3.5 py-1.5 rounded-md hover:bg-[#181c25] text-slate-300 hover:text-white transition-colors flex items-center gap-1.5"
                    >
                        <span>🏆</span>
                        Leaderboard
                    </Link>

                    {isAuthenticated && (
                        <Link
                            href="/submissions"
                            className="px-3.5 py-1.5 rounded-md hover:bg-[#181c25] text-slate-300 hover:text-white transition-colors flex items-center gap-1.5"
                        >
                            <span>📜</span>
                            My Submissions
                        </Link>
                    )}

                    <div className="h-4 w-[1px] bg-[#262d3d] mx-1 sm:mx-2" />

                    {isAuthenticated && user ? (
                        <div className="flex items-center gap-2">
                            <span className="px-2.5 py-1 rounded bg-[#181c25] border border-[#262d3d] text-[#38bdf8] text-[11px] font-semibold">
                                {user.team_id || user.username}
                            </span>
                            <button
                                onClick={logout}
                                className="px-2.5 py-1 rounded hover:bg-rose-950/40 text-slate-400 hover:text-rose-400 border border-transparent hover:border-rose-900/40 transition-colors"
                            >
                                Sign Out
                            </button>
                        </div>
                    ) : (
                        <div className="flex items-center gap-1.5">
                            <Link
                                href="/login"
                                className="px-3 py-1.5 rounded text-slate-300 hover:text-white hover:bg-[#181c25] transition-colors"
                            >
                                Sign In
                            </Link>
                            <Link
                                href="/signup"
                                className="px-3 py-1.5 rounded bg-blue-600 hover:bg-blue-500 text-white font-medium transition-colors"
                            >
                                Sign Up
                            </Link>
                        </div>
                    )}
                </div>
            </div>
        </nav>
    );
}

