'use client';

import React, { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import '../globals.css';

interface LeaderboardEntry {
    rank: number;
    team_id: string;
    score: number;
    correctness: string;
    tps: number;
    p99_us: number;
    status: string;
}

export default function LeaderboardPage() {
    const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
    const [loading, setLoading] = useState(true);
    const [isRefreshing, setIsRefreshing] = useState(false);
    const [lastUpdated, setLastUpdated] = useState<string>('');
    const [fetchError, setFetchError] = useState<string | null>(null);

    const fetchLeaderboard = useCallback(async (isManual = false) => {
        if (isManual) setIsRefreshing(true);
        try {
            const res = await fetch('http://localhost:8080/api/leaderboard', {
                cache: 'no-store',
            });
            if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
            const data: LeaderboardEntry[] = await res.json();
            setEntries(Array.isArray(data) ? data : []);
            setLastUpdated(new Date().toLocaleTimeString());
            setFetchError(null);
        } catch (err: any) {
            console.error('Failed to fetch leaderboard:', err);
            setFetchError(err.message || 'Cannot reach API server');
        } finally {
            setLoading(false);
            if (isManual) setIsRefreshing(false);
        }
    }, []);

    // Initial fetch & 3-second auto-polling loop
    useEffect(() => {
        fetchLeaderboard();
        const timer = setInterval(() => {
            fetchLeaderboard();
        }, 3000);
        return () => clearInterval(timer);
    }, [fetchLeaderboard]);

    const getRankBadge = (rank: number) => {
        if (rank === 1) {
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-bold bg-amber-500/15 text-amber-400 border border-amber-500/30">
                    🥇 #1
                </span>
            );
        }
        if (rank === 2) {
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-bold bg-slate-300/15 text-slate-300 border border-slate-300/30">
                    🥈 #2
                </span>
            );
        }
        if (rank === 3) {
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-bold bg-amber-700/20 text-amber-600 border border-amber-700/30">
                    🥉 #3
                </span>
            );
        }
        return <span className="text-[#94a3b8] font-mono font-medium">#{rank}</span>;
    };

    return (
        <div className="min-h-screen p-8 max-w-7xl mx-auto">
            {/* Header with live status and refresh controls */}
            <header className="mb-8 border-b border-[#262d3d] pb-6 flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold tracking-tight text-white">Global Leaderboard</h1>
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono bg-emerald-950/60 text-emerald-400 border border-emerald-800/60">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                            LIVE REDIS SYNC
                        </span>
                    </div>
                    <p className="text-sm text-[#94a3b8] mt-1.5">
                        Rankings calculated via composite score: <code className="text-blue-400 font-mono text-xs">Score = TPS / p99(ms)</code> with correctness gate enforcement.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    {lastUpdated && (
                        <span className="text-xs font-mono text-[#94a3b8] hidden sm:inline">
                            Synced at {lastUpdated}
                        </span>
                    )}
                    <button
                        onClick={() => fetchLeaderboard(true)}
                        disabled={isRefreshing}
                        className="px-3.5 py-1.5 text-xs font-mono rounded bg-[#181c25] hover:bg-[#202634] text-slate-200 border border-[#262d3d] transition-colors flex items-center gap-1.5 disabled:opacity-50"
                    >
                        <svg className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        {isRefreshing ? 'Refreshing...' : 'Refresh'}
                    </button>
                    <Link
                        href="/submit"
                        className="px-3.5 py-1.5 text-xs font-mono font-medium rounded bg-blue-600 hover:bg-blue-500 text-white transition-colors"
                    >
                        + Submit Engine
                    </Link>
                </div>
            </header>

            {fetchError && (
                <div className="mb-6 p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 flex items-center justify-between text-xs text-rose-300 font-mono">
                    <div className="flex items-center gap-2">
                        <span>⚠️</span>
                        <span>Backend Connection Error: {fetchError}. Ensure <code className="text-rose-200">go run main.go</code> is active on port 8080.</span>
                    </div>
                    <button
                        onClick={() => fetchLeaderboard(true)}
                        className="underline hover:text-white"
                    >
                        Retry
                    </button>
                </div>
            )}

            {/* Table */}
            <div className="bg-[#12151c] border border-[#262d3d] rounded-lg shadow-xl overflow-hidden">
                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr className="border-b border-[#262d3d] text-xs font-mono uppercase text-[#94a3b8] bg-[#181c25]/60">
                            <th className="py-3.5 px-6">Rank</th>
                            <th className="py-3.5 px-6">Team Identifier</th>
                            <th className="py-3.5 px-6">Composite Score</th>
                            <th className="py-3.5 px-6">Correctness</th>
                            <th className="py-3.5 px-6">Throughput (TPS)</th>
                            <th className="py-3.5 px-6">p99 Latency</th>
                            <th className="py-3.5 px-6">Status</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-[#262d3d] font-mono text-sm">
                        {loading && entries.length === 0 ? (
                            <tr>
                                <td colSpan={7} className="py-12 text-center text-[#94a3b8]">
                                    <div className="flex flex-col items-center justify-center gap-2">
                                        <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
                                        <span>Loading live rankings from Redis...</span>
                                    </div>
                                </td>
                            </tr>
                        ) : entries.length === 0 ? (
                            <tr>
                                <td colSpan={7} className="py-14 text-center text-[#94a3b8]">
                                    <div className="max-w-md mx-auto flex flex-col items-center gap-3">
                                        <div className="text-3xl">🏁</div>
                                        <h3 className="text-base font-semibold text-white font-sans">No Benchmark Runs Recorded Yet</h3>
                                        <p className="text-xs text-[#94a3b8]">
                                            Deploy and stress-test your containerized matching engine to claim the top spot on the leaderboard.
                                        </p>
                                        <Link
                                            href="/submit"
                                            className="mt-2 px-4 py-2 text-xs font-mono rounded bg-blue-600 hover:bg-blue-500 text-white font-semibold transition-colors"
                                        >
                                            Submit Your First Engine →
                                        </Link>
                                    </div>
                                </td>
                            </tr>
                        ) : (
                            entries.map((entry) => (
                                <tr key={`${entry.team_id}-${entry.rank}`} className="hover:bg-[#181c25]/40 transition-colors">
                                    <td className="py-4 px-6">{getRankBadge(entry.rank)}</td>
                                    <td className="py-4 px-6 font-semibold text-white">{entry.team_id}</td>
                                    <td className="py-4 px-6 text-blue-400 font-bold text-base">
                                        {entry.score.toFixed(2)}
                                    </td>
                                    <td className="py-4 px-6">
                                        <span className={`px-2 py-0.5 text-xs rounded font-semibold ${
                                            entry.correctness === 'PASS' 
                                                ? 'bg-emerald-950/80 text-emerald-400 border border-emerald-800' 
                                                : 'bg-rose-950/80 text-rose-400 border border-rose-800'
                                        }`}>
                                            {entry.correctness}
                                        </span>
                                    </td>
                                    <td className="py-4 px-6 text-slate-200">
                                        {entry.tps.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}
                                    </td>
                                    <td className="py-4 px-6 text-slate-200">
                                        {entry.p99_us.toFixed(2)} µs
                                    </td>
                                    <td className="py-4 px-6">
                                        <span className={`text-xs uppercase font-semibold ${
                                            entry.status === 'COMPLETED' ? 'text-emerald-400' : 'text-[#94a3b8]'
                                        }`}>
                                            {entry.status}
                                        </span>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
}