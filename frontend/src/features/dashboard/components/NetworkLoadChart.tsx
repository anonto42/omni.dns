import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip,
  ResponsiveContainer, PieChart, Pie, Cell,
} from 'recharts'
import { BarChart3 } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useNetworkLoad, CHART_POINTS, TOOLTIP_STYLE } from '../hooks/useNetworkLoad'

/** Presenter component — renders the live area chart + donut breakdown. */
export default function NetworkLoadChart() {
  const {
    data,
    loading,
    cumulative,
    cumTotal,
    cumPctBlocked,
    cumPctAllowed,
    pieData,
    hasData,
  } = useNetworkLoad()

  return (
    <Card className="overflow-hidden shadow-sm" data-tour="chart-queries">
      <CardHeader className="pb-0 pt-4 px-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          {/* Title */}
          <div>
            <CardTitle className="text-lg flex items-center gap-2 font-bold tracking-tight text-foreground">
              <BarChart3 className="h-5 w-5 text-primary" />
              Network Load
            </CardTitle>
            <CardDescription className="text-[10px] font-bold uppercase tracking-widest mt-0.5">
              Live query rate · updates every 3s
            </CardDescription>
          </div>

          {/* Analytics pills */}
          <div className="flex flex-wrap gap-2">
            {[
              { label: 'Total Queries',    value: cumTotal.toLocaleString(),          color: 'text-foreground',                   bg: 'bg-muted/50' },
              { label: 'Blocked',          value: cumulative.blocked.toLocaleString(), color: 'text-rose-500',                     bg: 'bg-rose-500/10' },
              { label: '% Blocked',        value: `${cumPctBlocked}%`,                color: 'text-rose-500',                     bg: 'bg-rose-500/10' },
              { label: '% Allowed',        value: `${cumPctAllowed}%`,                color: 'text-emerald-600 dark:text-emerald-400', bg: 'bg-emerald-500/10' },
            ].map(pill => (
              <div key={pill.label} className={`flex flex-col items-center px-3 py-1.5 ${pill.bg}`}>
                <span className={`text-sm font-bold tabular-nums ${pill.color}`}>{loading ? '—' : pill.value}</span>
                <span className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">{pill.label}</span>
              </div>
            ))}
          </div>
        </div>
      </CardHeader>

      <CardContent className="pb-4 px-2 pt-2">
        {loading ? (
          <Skeleton className="w-full h-[200px]" />
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-0">
            {/* Area chart — 2/3 width */}
            <div className="lg:col-span-2">
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={data} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="gradAllowed" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%"  stopColor="#22c55e" stopOpacity={0.2} />
                      <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="gradBlocked" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%"  stopColor="#f43f5e" stopOpacity={0.2} />
                      <stop offset="95%" stopColor="#f43f5e" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="gradCached" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%"  stopColor="var(--primary)" stopOpacity={0.25} />
                      <stop offset="95%" stopColor="var(--primary)" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="currentColor" strokeOpacity={0.06} vertical={false} />
                  <XAxis
                    dataKey="time"
                    tick={{ fontSize: 9, fontFamily: 'monospace', fill: 'currentColor', opacity: 0.4 }}
                    tickLine={false}
                    axisLine={false}
                    interval={Math.floor(CHART_POINTS / 5)}
                  />
                  <YAxis
                    tick={{ fontSize: 9, fontFamily: 'monospace', fill: 'currentColor', opacity: 0.4 }}
                    tickLine={false}
                    axisLine={false}
                    allowDecimals={false}
                    width={30}
                    label={{ value: 'req/3s', angle: -90, position: 'insideLeft', offset: 16, style: { fontSize: 8, fill: 'currentColor', opacity: 0.3, fontFamily: 'monospace' } }}
                  />
                  <RechartsTooltip
                    contentStyle={TOOLTIP_STYLE}
                    labelStyle={{ color: 'var(--muted-foreground)', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', fontSize: 9 }}
                    itemStyle={{ color: 'var(--foreground)' }}
                    cursor={{ stroke: 'var(--primary)', strokeWidth: 1, strokeDasharray: '4 2' }}
                    formatter={(value: number, name: string) => [`${value} req`, name]}
                  />
                  <Area type="monotone" dataKey="allowed" name="Allowed" stroke="#22c55e" strokeWidth={1.5} fill="url(#gradAllowed)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} />
                  <Area type="monotone" dataKey="blocked" name="Blocked" stroke="#f43f5e" strokeWidth={1.5} fill="url(#gradBlocked)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} />
                  <Area type="monotone" dataKey="cached"  name="Cached"  stroke="var(--primary)" strokeWidth={1.5} fill="url(#gradCached)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} />
                </AreaChart>
              </ResponsiveContainer>
            </div>

            {/* Donut + legend — 1/3 width */}
            <div className="flex flex-col items-center justify-center px-2 py-2">
              {hasData ? (
                <>
                  <ResponsiveContainer width="100%" height={140}>
                    <PieChart>
                      <Pie
                        data={pieData}
                        cx="50%"
                        cy="50%"
                        innerRadius={42}
                        outerRadius={60}
                        paddingAngle={2}
                        dataKey="value"
                        strokeWidth={0}
                      >
                        {pieData.map((entry, i) => (
                          <Cell key={i} fill={entry.color} />
                        ))}
                      </Pie>
                      <RechartsTooltip
                        contentStyle={TOOLTIP_STYLE}
                        itemStyle={{ color: 'var(--foreground)' }}
                        formatter={(value: number, name: string) => [value.toLocaleString(), name]}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="w-full space-y-1.5 px-1">
                    {pieData.map(d => (
                      <div key={d.name} className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-1.5 min-w-0">
                          <div className="w-2 h-2 shrink-0" style={{ background: d.color }} />
                          <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground truncate">{d.name}</span>
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0">
                          <span className="text-[10px] font-bold text-foreground tabular-nums">{d.value.toLocaleString()}</span>
                          <span className="text-[9px] text-muted-foreground tabular-nums">({d.pct}%)</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              ) : (
                <div className="flex flex-col items-center gap-2 py-10">
                  <div className="w-16 h-16 bg-muted/30 flex items-center justify-center">
                    <BarChart3 className="h-6 w-6 text-muted-foreground/30" />
                  </div>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground text-center">No traffic yet</p>
                  <p className="text-[9px] text-muted-foreground/60 text-center">Make a DNS query to see data</p>
                </div>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
