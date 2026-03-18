export const STATE_COLORS: Record<string, { bg: string; text: string; dot: string }> = {
  pending:            { bg: 'bg-blue-500/10', text: 'text-blue-400', dot: 'bg-blue-400' },
  waiting_for_worker: { bg: 'bg-violet-500/10', text: 'text-violet-400', dot: 'bg-violet-400' },
  queued:             { bg: 'bg-blue-500/10', text: 'text-blue-300', dot: 'bg-blue-300' },
  running:            { bg: 'bg-indigo-500/15', text: 'text-indigo-400', dot: 'bg-indigo-400' },
  retrying:           { bg: 'bg-orange-500/10', text: 'text-orange-400', dot: 'bg-orange-400' },
  kill_requested:     { bg: 'bg-red-500/10', text: 'text-red-400', dot: 'bg-red-400' },
  completed:          { bg: 'bg-emerald-500/10', text: 'text-emerald-400', dot: 'bg-emerald-400' },
  failed:             { bg: 'bg-red-500/10', text: 'text-red-400', dot: 'bg-red-400' },
  hung:               { bg: 'bg-amber-500/10', text: 'text-amber-400', dot: 'bg-amber-400' },
  killed:             { bg: 'bg-red-500/10', text: 'text-red-300', dot: 'bg-red-300' },
  skipped:            { bg: 'bg-zinc-500/10', text: 'text-zinc-400', dot: 'bg-zinc-400' },
  cancelled:          { bg: 'bg-zinc-500/10', text: 'text-zinc-500', dot: 'bg-zinc-500' },
  paused:             { bg: 'bg-yellow-500/10', text: 'text-yellow-400', dot: 'bg-yellow-400' },
}

export const METHOD_ICONS: Record<string, string> = {
  http: 'Globe',
  ssh: 'KeyRound',
  ssm: 'Cloud',
  k8s: 'Container',
}

export const SCHEDULE_LABELS: Record<string, string> = {
  cron: 'Cron',
  fixed_delay: 'Fixed Delay',
  on_demand: 'On Demand',
}

export const ORIGIN_LABELS: Record<string, string> = {
  cron: 'Cron',
  fixed_delay: 'Delay',
  manual: 'Manual',
  one_time: 'One-time',
  recovery: 'Recovery',
  dependency: 'Dependency',
  replay: 'Replay',
}
