'use client';

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import { useAuth } from '@/context/AuthContext';
import '../globals.css';

export default function SubmitPage() {
    const { user, token, isAuthenticated } = useAuth();
    const [teamName, setTeamName] = useState('');
    const [dockerImage, setDockerImage] = useState('');
    const [loading, setLoading] = useState(false);
    const [responseMsg, setResponseMsg] = useState<{ type: 'success' | 'error'; text: string; runId?: string } | null>(null);

    // Auto-fill team identifier when user is logged in
    useEffect(() => {
        if (user && user.team_id) {
            setTeamName(user.team_id);
        }
    }, [user]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setResponseMsg(null);

        try {
            const headers: Record<string, string> = {
                'Content-Type': 'application/json',
            };
            if (token) {
                headers['Authorization'] = `Bearer ${token}`;
            }

            const res = await fetch('http://localhost:8080/api/submit', {
                method: 'POST',
                headers,
                body: JSON.stringify({
                    team_id: teamName.trim(),
                    image: dockerImage.trim(),
                }),
            });

            if (!res.ok) {
                const errText = await res.text();
                throw new Error(errText || 'Failed to dispatch benchmark job.');
            }

            const data = await res.json();
            setResponseMsg({
                type: 'success',
                text: `Submission dispatched successfully! Run ID: ${data.run_id} (Allocated Engine Port: ${data.expose_port})`,
                runId: data.run_id,
            });
            setDockerImage('');
        } catch (err: any) {
            setResponseMsg({
                type: 'error',
                text: err.message || 'Server connection error.',
            });
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen p-8 max-w-4xl mx-auto">
            <header className="mb-8 border-b border-[#262d3d] pb-4">
                <h1 className="text-2xl font-semibold tracking-tight">Engine Submission Portal</h1>
                <p className="text-sm text-[#94a3b8] mt-1">Deploy your containerized matching engine for automated stress testing.</p>
            </header>

            {!isAuthenticated && (
                <div className="mb-6 p-4 rounded-lg bg-amber-950/30 border border-amber-800/40 text-amber-300 text-xs flex items-center justify-between">
                    <div>
                        <span className="font-bold">Notice:</span> You are submitting without signing in. Runs will not be linked to your persistent account history.
                    </div>
                    <Link
                        href="/login"
                        className="px-3 py-1 bg-amber-600 hover:bg-amber-500 text-white rounded font-mono font-semibold transition-colors shrink-0 ml-4"
                    >
                        Sign In Now
                    </Link>
                </div>
            )}

            <div className="bg-[#12151c] border border-[#262d3d] rounded-lg p-6 shadow-xl">
                <form onSubmit={handleSubmit} className="space-y-5">
                    <div>
                        <label className="block text-xs font-mono uppercase tracking-wider text-[#94a3b8] mb-2">
                            Team Identifier
                        </label>
                        <input
                            type="text"
                            required
                            placeholder="e.g., team-alpha"
                            value={teamName}
                            onChange={(e) => setTeamName(e.target.value)}
                            className="w-full bg-[#090a0f] border border-[#262d3d] rounded px-4 py-2.5 text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
                        />
                    </div>

                    <div>
                        <label className="block text-xs font-mono uppercase tracking-wider text-[#94a3b8] mb-2">
                            Docker Image URL (Registry)
                        </label>
                        <input
                            type="text"
                            required
                            placeholder="e.g., username/matching-engine:v1 or mock-exchange:latest"
                            value={dockerImage}
                            onChange={(e) => setDockerImage(e.target.value)}
                            className="w-full bg-[#090a0f] border border-[#262d3d] rounded px-4 py-2.5 text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full bg-blue-600 hover:bg-blue-500 text-white font-medium py-2.5 px-4 rounded text-sm transition-colors duration-150 disabled:opacity-50"
                    >
                        {loading ? 'Registering Run & Spawning Sandbox...' : 'Deploy & Run Benchmark'}
                    </button>
                </form>

                {responseMsg && (
                    <div
                        className={`mt-6 p-4 rounded text-sm font-mono border flex items-center justify-between ${responseMsg.type === 'success'
                            ? 'bg-emerald-950/30 border-emerald-800/50 text-emerald-400'
                            : 'bg-rose-950/30 border-rose-800/50 text-rose-400'
                            }`}
                    >
                        <span>{responseMsg.text}</span>
                        {responseMsg.type === 'success' && isAuthenticated && (
                            <Link
                                href="/submissions"
                                className="px-3 py-1 bg-emerald-600/30 hover:bg-emerald-600/50 text-emerald-300 border border-emerald-500/30 rounded text-xs transition-colors shrink-0 ml-4"
                            >
                                View History &rarr;
                            </Link>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
}
