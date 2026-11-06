'use client';

import React, { useEffect, useState, useRef } from 'react';
import Link from 'next/link';
import '../globals.css';

interface MetricPoint {
    time: string;
    t: number; // seconds since first sample
    tps: number;
    p50Us: number;
    p90Us: number;
    p99Us: number;
    teamId: string;
    runId: string;
    totalSent: number;
    totalAcks: number;
}

export default function LiveMonitorPage() {
    const [metricsHistory, setMetricsHistory] = useState<MetricPoint[]>([]);
    const [connectionStatus, setConnectionStatus] = useState<'connected' | 'connecting' | 'disconnected'>('connecting');
    const [currentTPS, setCurrentTPS] = useState(0);
    const [peakTPS, setPeakTPS] = useState(0);
    const [currentP50, setCurrentP50] = useState(0);
    const [currentP90, setCurrentP90] = useState(0);
    const [currentP99, setCurrentP99] = useState(0);
    const [activeTeam, setActiveTeam] = useState<string>('—');
    const [activeRunId, setActiveRunId] = useState<string>('—');
    const [totalSent, setTotalSent] = useState<number>(0);
    const [totalAcks, setTotalAcks] = useState<number>(0);

    const [hoverIdx, setHoverIdx] = useState<number | null>(null);

    const startTimeRef = useRef<number | null>(null);
    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimerRef = useRef<NodeJS.Timeout | null>(null);

    useEffect(() => {
        let isUnmounted = false;

        const connect = () => {
            if (isUnmounted) return;
            setConnectionStatus('connecting');

            const ws = new WebSocket('ws://localhost:8080/ws/metrics');
            wsRef.current = ws;

            ws.onopen = () => {
                if (isUnmounted) return;
                setConnectionStatus('connected');
                if (reconnectTimerRef.current) {
                    clearTimeout(reconnectTimerRef.current);
                    reconnectTimerRef.current = null;
                }
            };

            ws.onclose = () => {
                if (isUnmounted) return;
                setConnectionStatus('disconnected');
                // Schedule auto-reconnect after 2.5 seconds
                if (!reconnectTimerRef.current) {
                    reconnectTimerRef.current = setTimeout(() => {
                        reconnectTimerRef.current = null;
                        connect();
                    }, 2500);
                }
            };

            ws.onerror = () => {
                ws.close();
            };

            ws.onmessage = (event) => {
                if (isUnmounted) return;
                try {
                    const data = JSON.parse(event.data);
                    const now = Date.now();
                    if (startTimeRef.current === null) startTimeRef.current = now;
                    const point: MetricPoint = {
                        t: (now - startTimeRef.current) / 1000,
                        time: new Date().toLocaleTimeString(),
                        tps: data.tps || 0,
                        p50Us: data.p50_us || 0,
                        p90Us: data.p90_us || 0,
                        p99Us: data.p99_us || 0,
                        teamId: data.team_id || '',
                        runId: data.run_id || '',
                        totalSent: data.total_sent || 0,
                        totalAcks: data.total_acks || 0,
                    };

                    setCurrentTPS(point.tps);
                    setCurrentP50(point.p50Us);
                    setCurrentP90(point.p90Us);
                    setCurrentP99(point.p99Us);
                    if (point.teamId) setActiveTeam(point.teamId);
                    if (point.runId) setActiveRunId(point.runId);
                    if (point.totalSent) setTotalSent(point.totalSent);
                    if (point.totalAcks) setTotalAcks(point.totalAcks);

                    setPeakTPS((prev) => Math.max(prev, point.tps));
                    setMetricsHistory((prev) => [...prev.slice(-300), point]); // Keep recent history; the chart shows the last WINDOW_SEC seconds
                } catch (e) {
                    console.error('Failed to parse telemetry message', e);
                }
            };
        };

        connect();

        return () => {
            isUnmounted = true;
            if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
            if (wsRef.current) wsRef.current.close();
        };
    }, []);

    // Calculate dynamic ceiling for the sparkline chart
    // X axis is real elapsed time (seconds). Starts at 0 on the left, fills rightwards,
    // then scrolls once elapsed time exceeds the window.
    const WINDOW_SEC = 30;
    const n = metricsHistory.length;
    const latestT = n ? metricsHistory[n - 1].t : 0;
    const xMax = Math.max(WINDOW_SEC, latestT);
    const xMin = xMax - WINDOW_SEC;
    const visible = metricsHistory.filter((pt) => pt.t >= xMin);
    const last = visible.length ? visible[visible.length - 1] : null;

    // Dynamic ceiling for the chart
    const maxGraphTPS = Math.max(4000, ...visible.map((pt) => pt.tps));

    // Line chart geometry (SVG viewBox is 0..100 on both axes)
    const xAt = (t: number) => ((t - xMin) / WINDOW_SEC) * 100;
    const yAt = (tps: number) => 100 - Math.min(100, (tps / maxGraphTPS) * 100);
    const linePath = visible
        .map((pt, i) => `${i === 0 ? 'M' : 'L'}${xAt(pt.t).toFixed(2)},${yAt(pt.tps).toFixed(2)}`)
        .join(' ');
    const areaPath =
        visible.length > 1
            ? `${linePath} L${xAt(visible[visible.length - 1].t).toFixed(2)},100 L${xAt(visible[0].t).toFixed(2)},100 Z`
            : '';
    const hoverPt = hoverIdx !== null ? visible[hoverIdx] ?? null : null;
    const xTicks = Array.from({ length: 7 }, (_, i) => Math.round(xMin + (i * WINDOW_SEC) / 6));

    const handleChartMove = (e: React.MouseEvent<HTMLDivElement>) => {
        if (visible.length === 0) return;
        const rect = e.currentTarget.getBoundingClientRect();
        const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
        const tHover = xMin + ratio * WINDOW_SEC;
        let best = 0;
        for (let i = 1; i < visible.length; i++) {
            if (Math.abs(visible[i].t - tHover) < Math.abs(visible[best].t - tHover)) best = i;
        }
        setHoverIdx(best);
    };

    const clearHistory = () => {
        startTimeRef.current = null;
        setMetricsHistory([]);
        setPeakTPS(0);
        setCurrentTPS(0);
        setCurrentP50(0);
        setCurrentP90(0);
        setCurrentP99(0);
    };

    return (
        <div className="min-h-screen p-8 max-w-7xl mx-auto">
            {/* Header */}
            <header className="flex flex-col md:flex-row md:items-center justify-between mb-8 border-b border-[#262d3d] pb-6 gap-4">
                <div>
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold tracking-tight text-white">Live Telemetry Monitor</h1>
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono bg-[#181c25] text-slate-300 border border-[#262d3d]">
                            RUN: <strong className="text-blue-400">{activeRunId !== '—' ? activeRunId.slice(0, 12) : 'STANDBY'}</strong>
                        </span>
                    </div>
                    <p className="text-sm text-[#94a3b8] mt-1.5">
                        Real-time stress test observability streamed via Redis Pub/Sub directly to this client.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <div className="flex items-center space-x-2 font-mono text-xs px-3 py-1.5 rounded bg-[#12151c] border border-[#262d3d]">
                        <span
                            className={`w-2 h-2 rounded-full ${connectionStatus === 'connected'
                                ? 'bg-emerald-500 animate-pulse'
                                : connectionStatus === 'connecting'
                                    ? 'bg-amber-500 animate-pulse'
                                    : 'bg-rose-500'
                                }`}
                        />
                        <span className="text-[#94a3b8]">
                            {connectionStatus === 'connected'
                                ? 'WS CONNECTED'
                                : connectionStatus === 'connecting'
                                    ? 'RECONNECTING...'
                                    : 'DISCONNECTED'}
                        </span>
                    </div>

                    <button
                        onClick={clearHistory}
                        className="px-3 py-1.5 text-xs font-mono rounded bg-[#181c25] hover:bg-[#202634] text-slate-300 border border-[#262d3d] transition-colors"
                    >
                        Clear Buffer
                    </button>

                    <Link
                        href="/leaderboard"
                        className="px-3 py-1.5 text-xs font-mono font-medium rounded bg-blue-600 hover:bg-blue-500 text-white transition-colors"
                    >
                        View Leaderboard →
                    </Link>
                </div>
            </header>

            {/* Top KPI Cards Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
                {/* Real-time Throughput */}
                <div className="bg-[#12151c] border border-[#262d3d] rounded-lg p-5">
                    <div className="flex items-center justify-between">
                        <span className="text-xs font-mono uppercase text-[#94a3b8]">Live Throughput</span>
                        <span className="text-[11px] font-mono text-slate-400">Peak: {peakTPS.toLocaleString(undefined, { maximumFractionDigits: 0 })}</span>
                    </div>
                    <div className="text-3xl font-mono font-bold mt-2 text-white">
                        {currentTPS.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}
                        <span className="text-xs font-normal text-[#94a3b8] ml-2">TPS</span>
                    </div>
                </div>

                {/* p99 Tail Latency */}
                <div className="bg-[#12151c] border border-[#262d3d] rounded-lg p-5">
                    <span className="text-xs font-mono uppercase text-[#94a3b8]">p99 Tail Latency</span>
                    <div className="text-3xl font-mono font-bold mt-2 text-rose-400">
                        {currentP99.toFixed(2)}
                        <span className="text-xs font-normal text-[#94a3b8] ml-2">µs</span>
                    </div>
                </div>

                {/* p50 / p90 Latencies */}
                <div className="bg-[#12151c] border border-[#262d3d] rounded-lg p-5">
                    <span className="text-xs font-mono uppercase text-[#94a3b8]">Median & p90</span>
                    <div className="flex items-baseline gap-3 mt-2 font-mono">
                        <div>
                            <span className="text-xs text-[#94a3b8]">p50: </span>
                            <span className="text-xl font-bold text-emerald-400">{currentP50.toFixed(1)} µs</span>
                        </div>
                        <span className="text-[#262d3d]">|</span>
                        <div>
                            <span className="text-xs text-[#94a3b8]">p90: </span>
                            <span className="text-xl font-bold text-amber-400">{currentP90.toFixed(1)} µs</span>
                        </div>
                    </div>
                </div>

                {/* Engine Team & Total Acks */}
                <div className="bg-[#12151c] border border-[#262d3d] rounded-lg p-5">
                    <span className="text-xs font-mono uppercase text-[#94a3b8]">Active Engine</span>
                    <div className="text-xl font-mono font-bold mt-2 text-blue-400 truncate" title={activeTeam}>
                        {activeTeam}
                    </div>
                    <div className="text-[11px] font-mono text-[#94a3b8] mt-1">
                        Acks: <span className="text-white">{totalAcks.toLocaleString()}</span> / Sent: <span className="text-white">{totalSent.toLocaleString()}</span>
                    </div>
                </div>
            </div>

            {/* Telemetry Stream Sparkline */}
            <div className="bg-[#12151c] border border-[#262d3d] rounded-lg p-6 shadow-xl">
                <div className="flex items-center justify-between mb-4">
                    <div>
                        <h2 className="text-sm font-mono uppercase tracking-wider text-white">Live Rolling TPS Stream</h2>
                        <p className="text-xs text-[#94a3b8] mt-0.5">TPS vs. elapsed time, showing the latest 30 seconds of telemetry from the ingester.</p>
                    </div>
                    <div className="text-xs font-mono text-slate-400 bg-[#181c25] px-2.5 py-1 rounded border border-[#262d3d]">
                        Scale: 0 — {maxGraphTPS.toLocaleString()} TPS
                    </div>
                </div>

                <div className="flex h-56 w-full gap-2">
                    {/* Y-axis labels */}
                    <div className="flex flex-col justify-between text-[10px] font-mono text-[#94a3b8] text-right w-12 py-0.5">
                        {[1, 0.75, 0.5, 0.25, 0].map((f) => (
                            <span key={f}>{Math.round(maxGraphTPS * f).toLocaleString()}</span>
                        ))}
                    </div>

                    {/* Plot area */}
                    <div
                        className="relative flex-1 border-b border-l border-[#262d3d] bg-[#0d0f14]/80 rounded cursor-crosshair"
                        onMouseMove={handleChartMove}
                        onMouseLeave={() => setHoverIdx(null)}
                    >
                        {metricsHistory.length === 0 ? (
                            <div className="w-full h-full flex flex-col items-center justify-center text-xs font-mono text-[#94a3b8] gap-2">
                                <span className="w-2 h-2 rounded-full bg-blue-500 animate-ping" />
                                <span>Waiting for telemetry stream... Run a test from the Submit portal to see live traffic.</span>
                            </div>
                        ) : (
                            <>
                                {/* Horizontal gridlines */}
                                {[25, 50, 75].map((p) => (
                                    <div
                                        key={p}
                                        className="absolute left-0 right-0 border-t border-dashed border-[#262d3d]/70 pointer-events-none"
                                        style={{ top: `${p}%` }}
                                    />
                                ))}

                                <svg
                                    className="absolute inset-0 w-full h-full overflow-visible pointer-events-none"
                                    viewBox="0 0 100 100"
                                    preserveAspectRatio="none"
                                >
                                    <defs>
                                        <linearGradient id="tpsFill" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.35" />
                                            <stop offset="100%" stopColor="#3b82f6" stopOpacity="0" />
                                        </linearGradient>
                                    </defs>
                                    {areaPath && <path d={areaPath} fill="url(#tpsFill)" />}
                                    <path
                                        d={linePath}
                                        fill="none"
                                        stroke="#60a5fa"
                                        strokeWidth={2}
                                        strokeLinejoin="round"
                                        strokeLinecap="round"
                                        vectorEffect="non-scaling-stroke"
                                    />
                                </svg>

                                {/* Latest-point marker */}
                                {hoverPt === null && last && (
                                    <div
                                        className="absolute w-2.5 h-2.5 rounded-full bg-blue-400 border-2 border-[#0d0f14] -translate-x-1/2 -translate-y-1/2 pointer-events-none"
                                        style={{
                                            left: `${xAt(last.t)}%`,
                                            top: `${yAt(last.tps)}%`,
                                        }}
                                    />
                                )}

                                {/* Hover crosshair, dot and tooltip */}
                                {hoverPt && hoverIdx !== null && (
                                    <>
                                        <div
                                            className="absolute top-0 bottom-0 border-l border-slate-500/60 pointer-events-none"
                                            style={{ left: `${xAt(hoverPt.t)}%` }}
                                        />
                                        <div
                                            className="absolute w-2.5 h-2.5 rounded-full bg-blue-300 border-2 border-[#0d0f14] -translate-x-1/2 -translate-y-1/2 pointer-events-none"
                                            style={{ left: `${xAt(hoverPt.t)}%`, top: `${yAt(hoverPt.tps)}%` }}
                                        />
                                        <div
                                            className={`absolute bg-[#181c25] text-white font-mono text-[11px] p-2 rounded shadow-lg border border-[#262d3d] whitespace-nowrap z-20 pointer-events-none ${xAt(hoverPt.t) > 75 ? '-translate-x-full -ml-3' : 'ml-3'
                                                }`}
                                            style={{
                                                left: `${xAt(hoverPt.t)}%`,
                                                top: `${Math.min(yAt(hoverPt.tps), 60)}%`,
                                            }}
                                        >
                                            <div className="font-semibold text-blue-400">
                                                {hoverPt.tps.toLocaleString(undefined, { maximumFractionDigits: 1 })} TPS
                                            </div>
                                            <div className="text-slate-300">p99: {hoverPt.p99Us.toFixed(2)} µs</div>
                                            <div className="text-[#94a3b8] text-[10px]">t = {hoverPt.t.toFixed(1)} s · {hoverPt.time}</div>
                                        </div>
                                    </>
                                )}
                            </>
                        )}
                    </div>
                </div>

                <div className="ml-14 mt-2 flex justify-between text-[10px] font-mono text-[#94a3b8]">
                    {xTicks.map((t) => (
                        <span key={t}>{t}s</span>
                    ))}
                </div>
                <div className="text-center text-[11px] font-mono text-[#94a3b8] mt-1">Time (seconds)</div>
            </div>
        </div>
    );
}