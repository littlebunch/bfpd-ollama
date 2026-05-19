'use client';

import { useEffect, useState } from 'react';
import { checkHealth } from '@/lib/api';

type HealthState = 'checking' | 'healthy' | 'unhealthy';

export function HealthStatus() {
  const [status, setStatus] = useState<HealthState>('checking');

  useEffect(() => {
    const poll = async () => {
      const ok = await checkHealth();
      setStatus(ok ? 'healthy' : 'unhealthy');
    };

    poll();
    const interval = setInterval(poll, 30_000);
    return () => clearInterval(interval);
  }, []);

  const dot =
    status === 'healthy'
      ? 'bg-emerald-400'
      : status === 'unhealthy'
        ? 'bg-red-400'
        : 'bg-yellow-400 animate-pulse';

  const label =
    status === 'healthy'
      ? 'Backend online'
      : status === 'unhealthy'
        ? 'Backend offline'
        : 'Checking…';

  return (
    <div className="flex items-center gap-2 text-sm text-slate-400">
      <span className={`h-2 w-2 rounded-full ${dot}`} />
      <span>{label}</span>
    </div>
  );
}
