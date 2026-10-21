'use client';

import React, { useState } from 'react';

export default function SubmitPage() {
    const [teamName, setTeamName] = useState('');
    const [dockerImage, setDockerImage] = useState('');
    const [loading, setLoading] = useState(false);
    const [responseMsg, setResponseMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setResponseMsg(null);

        const submitId = `run-${Math.random().toString(36).substring(2, 9)}`;

        try {
            const res = await fetch('http://localhost:8080/api/run', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    team_id: teamName,
                    image: dockerImage,
                    submit_id: submitId,
                }),
            });

            if (!res.ok) throw new Error('Failed to dispatch benchmark job.');

            const data = await res.json();
            setResponseMsg({
                type: 'success',
                text: `Submission dispatched successfully! Run ID: ${data.run_id} (Port: ${data.expose_port})`,
            });
            setTeamName('');
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
                            placeholder="e.g., username/matching-engine:v1"
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
                        {loading ? 'Provisioning Sandbox & Queuing...' : 'Deploy & Run Benchmark'}
                    </button>
                </form>

                {responseMsg && (
                    <div
                        className={`mt-6 p-4 rounded text-sm font-mono border ${responseMsg.type === 'success'
                            ? 'bg-emerald-950/30 border-emerald-800/50 text-emerald-400'
                            : 'bg-rose-950/30 border-rose-800/50 text-rose-400'
                            }`}
                    >
                        {responseMsg.text}
                    </div>
                )}
            </div>
        </div>
    );
}