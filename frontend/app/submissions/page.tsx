'use client';

import React, { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/context/AuthContext';
import '../globals.css';

interface SubmissionItem {
    run_id: string;
    team_id: string;
    image: string;
    status: string;
    correctness: string;
    final_score: number;
    final_tps: number;
    p99_us: number;
    created_at: string;
}

export default function SubmissionsPage() {
    const { user, token, isAuthenticated, isLoading: authLoading } = useAuth();
    const [submissions, setSubmissions] = useState<SubmissionItem[]>([]);
    const [loading, setLoading] = useState(true);
    const [isRefreshing, setIsRefreshing] = useState(false);
    const [errorMsg, setErrorMsg] = useState<string | null>(null);

    const fetchSubmissions = useCallback(async (isManual = false) => {
        if (!token) return;
        if (isManual) setIsRefreshing(true);

        try {
            const res = await fetch('http://localhost:8080/api/submissions', {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                cache: 'no-store',
            });

            if (!res.ok) {
                throw new Error(`HTTP ${res.status}: ${res.statusText}`);
            }

            const data: SubmissionItem[] = await res.json();
            setSubmissions(Array.isArray(data) ? data : []);
            setErrorMsg(null);
        } catch (err: any) {
            console.error('Failed to fetch submissions:', err);
            setErrorMsg(err.message || 'Unable to retrieve submissions.');
        } finally {
            setLoading(false);
            if (isManual) setIsRefreshing(false);
        }
    }, [token]);

    useEffect(() => {
        if (token) {
            fetchSubmissions();
            // Polling interval to automatically update in-progress runs
            const interval = setInterval(() => {
                fetchSubmissions();
            }, 4000);
            return () => clearInterval(interval);
        } else if (!authLoading) {
            setLoading(false);
        }
    }, [token, authLoading, fetchSubmissions]);

    const getStatusBadge = (status: string) => {
        switch (status.toUpperCase()) {
            case 'COMPLETED':
                return (
                    <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-950/60 text-emerald-400 border border-emerald-800/60">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                        COMPLETED
                    </span>
                );
            case 'FAILED':
                return (
                    <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[11px] font-semibold bg-rose-950/60 text-rose-400 border border-rose-800/60">
                        <span className="w-1.5 h-1.5 rounded-full bg-rose-400" />
                        FAILED
                    </span>
                );
            case 'STRESS_TESTING':
            case 'RUNNING':
                return (
                    <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[11px] font-semibold bg-blue-950/60 text-blue-400 border border-blue-800/60">
                        <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-ping" />
                        BENCHMARKING
                    </span>
                );
            default:
                return (
                    <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[11px] font-semibold bg-amber-950/60 text-amber-400 border border-amber-800/60">
                        <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />
                        {status}
                    </span>
                );
        }
    };

    const getCorrectnessBadge = (correctness: string) => {
        switch (correctness.toUpperCase()) {
            case 'PASS':
                return <span className="text-emerald-400 font-bold">PASS</span>;
            case 'FAIL':
                return <span className="text-rose-400 font-bold">FAIL</span>;
            default:
                return <span className="text-slate-500 font-mono text-xs">PENDING</span>;
        }
    };

    if (authLoading) {
        return (
            <div className="min-h-[70vh] flex items-center justify-center text-sm font-mono text-slate-500">
                Checking authentication session...
            </div>
        );
    }

    if (!isAuthenticated) {
        return (
            <div className="min-h-[70vh] flex flex-col items-center justify-center p-6 text-center">
                <div className="w-12 h-12 rounded-xl bg-blue-600/20 border border-blue-500/30 flex items-center justify-center text-blue-400 font-mono font-bold text-xl mb-4">
                    🔒
                </div>
                <h2 className="text-xl font-bold text-white mb-2">Authentication Required</h2>
                <p className="text-sm text-[#94a3b8] max-w-sm mb-6">
                    Please sign in or create an account to view and persist your engine submission history.
                </p>
                <div className="flex gap-3">
                    <Link
                        href="/login"
                        className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md text-xs font-mono font-semibold transition-colors"
                    >
                        Sign In
                    </Link>
                    <Link
                        href="/signup"
                        className="px-4 py-2 bg-[#181c25] hover:bg-[#202632] border border-[#262d3d] text-slate-200 rounded-md text-xs font-mono transition-colors"
                    >
                        Sign Up
                    </Link>
                </div>
            </div>
        );
    }

    return (
        <div className="min-h-screen p-8 max-w-7xl mx-auto">
            {/* Header */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-[#262d3d] mb-6 gap-4">
                <div>
                    <h1 className="text-2xl font-semibold tracking-tight text-white flex items-center gap-2.5">
                        <span>📜</span>
                        Submission History
                    </h1>
                    <p className="text-xs text-[#94a3b8] mt-1 font-mono">
                        Persistent audit log for team: <span className="text-[#38bdf8] font-semibold">{user?.team_id || user?.username}</span>
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={() => fetchSubmissions(true)}
                        disabled={isRefreshing}
                        className="px-3.5 py-1.5 rounded-md bg-[#12151c] hover:bg-[#181c25] border border-[#262d3d] text-xs font-mono text-slate-300 transition-colors flex items-center gap-1.5 disabled:opacity-50"
                    >
                        <span className={isRefreshing ? 'animate-spin' : ''}>🔄</span>
                        {isRefreshing ? 'Refreshing...' : 'Refresh'}
                    </button>
                    <Link
                        href="/submit"
                        className="px-3.5 py-1.5 rounded-md bg-blue-600 hover:bg-blue-500 text-xs font-mono font-semibold text-white transition-colors"
                    >
                        + New Submission
                    </Link>
                </div>
            </div>

            {errorMsg && (
                <div className="mb-6 p-3.5 rounded-md bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">
                    {errorMsg}
                </div>
            )}

            {/* Submissions Table */}
            <div className="bg-[#12151c] border border-[#262d3d] rounded-lg overflow-hidden shadow-xl">
                {loading ? (
                    <div className="p-12 text-center text-sm font-mono text-[#94a3b8]">
                        Loading submission history from database...
                    </div>
                ) : submissions.length === 0 ? (
                    <div className="p-12 text-center">
                        <p className="text-sm text-[#94a3b8] font-mono">No previous submissions found for your account.</p>
                        <Link
                            href="/submit"
                            className="inline-block mt-4 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-mono font-medium transition-colors"
                        >
                            Deploy Your First Engine
                        </Link>
                    </div>
                ) : (
                    <div className="overflow-x-auto">
                        <table className="w-full text-left border-collapse text-xs font-mono">
                            <thead>
                                <tr className="border-b border-[#262d3d] bg-[#090a0f]/60 text-[#94a3b8]">
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider">Run ID</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider">Docker Image</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider">Status</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider text-center">Correctness</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider text-right">Unified Score</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider text-right">TPS</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider text-right">p99 Latency</th>
                                    <th className="py-3.5 px-4 font-medium uppercase tracking-wider text-right">Submitted At</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-[#262d3d]/60">
                                {submissions.map((item) => (
                                    <tr key={item.run_id} className="hover:bg-[#181c25]/50 transition-colors">
                                        <td className="py-3 px-4 font-bold text-white">
                                            {item.run_id}
                                        </td>
                                        <td className="py-3 px-4 text-[#94a3b8] max-w-xs truncate" title={item.image}>
                                            {item.image}
                                        </td>
                                        <td className="py-3 px-4">
                                            {getStatusBadge(item.status)}
                                        </td>
                                        <td className="py-3 px-4 text-center">
                                            {getCorrectnessBadge(item.correctness)}
                                        </td>
                                        <td className="py-3 px-4 text-right font-bold text-[#38bdf8]">
                                            {item.final_score > 0 ? item.final_score.toLocaleString(undefined, { maximumFractionDigits: 2 }) : '-'}
                                        </td>
                                        <td className="py-3 px-4 text-right text-slate-300">
                                            {item.final_tps > 0 ? `${item.final_tps.toLocaleString(undefined, { maximumFractionDigits: 0 })} /s` : '-'}
                                        </td>
                                        <td className="py-3 px-4 text-right text-slate-300">
                                            {item.p99_us > 0 ? `${item.p99_us.toLocaleString()} µs` : '-'}
                                        </td>
                                        <td className="py-3 px-4 text-right text-[#94a3b8]">
                                            {new Date(item.created_at).toLocaleString([], {
                                                month: 'short',
                                                day: 'numeric',
                                                hour: '2-digit',
                                                minute: '2-digit',
                                            })}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>
        </div>
    );
}

