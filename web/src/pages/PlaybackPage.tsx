import { useQuery } from "@tanstack/react-query";
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from "recharts";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { playbackService } from "@/services/playback";
import { Skeleton } from "@/components/ui/skeleton";

interface TopVideo {
  video_id: string;
  title: string;
  avg_watch_seconds: number;
  play_count: number;
}

interface UserRow {
  user_id: string;
  user_name: string;
  total_videos: number;
  total_seconds: number;
}

interface DayRow {
  date: string;
  play_count: number;
}

function fmtSeconds(s: number) {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}m ${sec}s`;
}

const TOOLTIP_STYLE = {
  contentStyle: {
    background: "hsl(0 0% 100%)",
    border: "1px solid hsl(36 14% 90%)",
    borderRadius: "10px",
    fontSize: "12px",
    color: "hsl(240 6% 12%)",
  },
};

export default function PlaybackPage() {
  const summary = useQuery({ queryKey: ["playback-summary"], queryFn: playbackService.summary });
  const topVideos = useQuery({ queryKey: ["playback-top"], queryFn: () => playbackService.topVideos("30d") });
  const byUser = useQuery({ queryKey: ["playback-by-user"], queryFn: playbackService.byUser });
  const daily = useQuery({ queryKey: ["playback-daily"], queryFn: playbackService.dailyTrend });

  const s = summary.data;

  const dailyData = (daily.data ?? []).map((r: DayRow) => ({
    date: r.date.slice(5), // "MM-DD"
    plays: r.play_count,
  }));

  return (
    <div>
      <PageHeader title="Playback Reports" subtitle="Video watching analytics" />

      {/* KPI cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
        {summary.isLoading ? Array(3).fill(0).map((_,i)=><Skeleton key={i} className="h-24 rounded-xl"/>) : (
          <>
            <StatCard label="Total Play Sessions" value={Number(s?.total_sessions ?? 0).toLocaleString()} />
            <StatCard label="Unique Viewers" value={Number(s?.unique_viewers ?? 0).toLocaleString()} />
            <StatCard label="Avg. Watch Duration" value={fmtSeconds(s?.avg_watch_seconds ?? 0)} />
          </>
        )}
      </div>

      {/* Daily trend chart */}
      <div className="rounded-xl border border-border bg-card p-5 mb-6">
        <h2 className="text-section-title mb-4">Daily Play Sessions (Last 30 Days)</h2>
        {daily.isLoading ? <Skeleton className="h-48" /> : (
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={dailyData} margin={{ top: 10, right: 20, left: 0, bottom: 5 }}>
              <defs>
                <linearGradient id="playsGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#b8965a" stopOpacity={0.25} />
                  <stop offset="95%" stopColor="#b8965a" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(36 14% 90%)" vertical={false} />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11, fill: "hsl(0 0% 42%)" }}
                axisLine={false}
                tickLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fontSize: 11, fill: "hsl(0 0% 42%)" }}
                axisLine={false}
                tickLine={false}
                allowDecimals={false}
              />
              <Tooltip {...TOOLTIP_STYLE} formatter={(v: number) => [v, "plays"]} />
              <Area
                type="monotone"
                dataKey="plays"
                stroke="#b8965a"
                strokeWidth={2}
                fill="url(#playsGradient)"
                dot={false}
                activeDot={{ r: 4, fill: "#b8965a", strokeWidth: 0 }}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Top videos table */}
      <div className="rounded-xl border border-border bg-card overflow-hidden mb-8">
        <div className="p-4 border-b border-border"><h2 className="text-section-title">Top Videos (Last 30 Days)</h2></div>
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">Rank</th>
              <th className="text-table-header text-left px-4 py-3">Video</th>
              <th className="text-table-header text-left px-4 py-3">Total Plays</th>
              <th className="text-table-header text-left px-4 py-3">Avg. Watch Time</th>
            </tr>
          </thead>
          <tbody>
            {topVideos.isLoading
              ? Array(3).fill(0).map((_,i)=><tr key={i}><td colSpan={4} className="px-4 py-3"><Skeleton className="h-4"/></td></tr>)
              : (topVideos.data ?? []).map((v: TopVideo, idx: number) => (
                <tr key={v.video_id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                  <td className="px-4 py-3 text-sm font-semibold text-primary">#{idx + 1}</td>
                  <td className="px-4 py-3 text-sm font-medium text-foreground">{v.title}</td>
                  <td className="px-4 py-3 text-sm text-foreground">{v.play_count}</td>
                  <td className="px-4 py-3 text-sm font-mono text-muted-foreground">{fmtSeconds(v.avg_watch_seconds)}</td>
                </tr>
              ))
            }
            {!topVideos.isLoading && (topVideos.data ?? []).length === 0 && (
              <tr><td colSpan={4} className="px-4 py-8 text-center text-sm text-muted-foreground">No playback data yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Per-user table */}
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <div className="p-4 border-b border-border"><h2 className="text-section-title">Per-User Playback</h2></div>
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">User</th>
              <th className="text-table-header text-left px-4 py-3">Videos Watched</th>
              <th className="text-table-header text-left px-4 py-3">Total Watch Time</th>
            </tr>
          </thead>
          <tbody>
            {byUser.isLoading
              ? Array(3).fill(0).map((_,i)=><tr key={i}><td colSpan={3} className="px-4 py-3"><Skeleton className="h-4"/></td></tr>)
              : (byUser.data ?? []).map((u: UserRow) => (
                <tr key={u.user_id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                  <td className="px-4 py-3 text-sm font-medium text-foreground">{u.user_name}</td>
                  <td className="px-4 py-3 text-sm text-foreground">{u.total_videos}</td>
                  <td className="px-4 py-3 text-sm text-muted-foreground">{fmtSeconds(u.total_seconds)}</td>
                </tr>
              ))
            }
            {!byUser.isLoading && (byUser.data ?? []).length === 0 && (
              <tr><td colSpan={3} className="px-4 py-8 text-center text-sm text-muted-foreground">No data yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
