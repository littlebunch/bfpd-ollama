'use client';

import { useState } from 'react';
import { RAGSource } from '@/lib/types';

interface SourceCardProps {
  source: RAGSource;
  index: number;
}

export function SourceCard({ source, index }: SourceCardProps) {
  const [expanded, setExpanded] = useState(false);

  const title =
    (source.metadata?.title as string) ||
    (source.metadata?.name as string) ||
    `Source ${index + 1}`;

  const calories = source.metadata?.calories as string | undefined;
  const score = (source.score * 100).toFixed(0);

  return (
    <div className="rounded-lg border border-slate-700 bg-slate-800/60 text-xs">
      <button
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center justify-between gap-2 px-3 py-2 text-left hover:bg-slate-700/40 transition-colors"
      >
        <div className="flex items-center gap-2 min-w-0">
          <span className="shrink-0 rounded bg-emerald-800 px-1.5 py-0.5 text-emerald-300 font-mono">
            [{index + 1}]
          </span>
          <span className="truncate text-slate-300 font-medium">{title}</span>
          {calories && (
            <span className="shrink-0 text-slate-500">{calories} kcal</span>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-slate-500">Score: {score}%</span>
          <span className="text-slate-500">{expanded ? '▲' : '▼'}</span>
        </div>
      </button>
      {expanded && (
        <div className="border-t border-slate-700 px-3 py-2 text-slate-400 leading-relaxed">
          {source.content}
          {Object.keys(source.metadata).length > 0 && (
            <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-slate-500">
              {Object.entries(source.metadata).map(([k, v]) => (
                <span key={k}>
                  <span className="text-slate-400">{k}:</span>{' '}
                  {String(v)}
                </span>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
