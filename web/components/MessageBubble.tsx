'use client';

import { UIMessage } from '@/lib/types';
import { SourceCard } from './SourceCard';

interface MessageBubbleProps {
  message: UIMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  const isUser = message.role === 'user';
  const isLoading = message.isLoading;

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[80%] space-y-2 ${isUser ? 'items-end' : 'items-start'} flex flex-col`}>
        {/* Avatar label */}
        <span className={`text-xs text-slate-500 px-1 ${isUser ? 'text-right' : 'text-left'}`}>
          {isUser ? 'You' : '🍽️ Nutrition AI'}
        </span>

        {/* Bubble */}
        <div
          className={`rounded-2xl px-4 py-3 text-sm leading-relaxed ${
            isUser
              ? 'bg-emerald-700 text-white rounded-br-sm'
              : 'bg-slate-700 text-slate-100 rounded-bl-sm'
          }`}
        >
          {isLoading ? (
            <span className="flex items-center gap-2 text-slate-400 italic">
              <span className="inline-block h-2 w-2 rounded-full bg-emerald-400 animate-bounce [animation-delay:0ms]" />
              <span className="inline-block h-2 w-2 rounded-full bg-emerald-400 animate-bounce [animation-delay:150ms]" />
              <span className="inline-block h-2 w-2 rounded-full bg-emerald-400 animate-bounce [animation-delay:300ms]" />
            </span>
          ) : (
            <span className="whitespace-pre-wrap">{message.content}</span>
          )}
        </div>

        {/* RAG Sources */}
        {!isLoading && message.sources && message.sources.length > 0 && (
          <div className="w-full space-y-1 mt-1">
            <p className="text-xs text-slate-500 px-1">📚 Sources</p>
            {message.sources.map((src, i) => (
              <SourceCard key={src.id || i} source={src} index={i} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
