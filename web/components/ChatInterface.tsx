'use client';

import { useEffect, useRef, useState } from 'react';
import { createConversation, sendChat } from '@/lib/api';
import { UIMessage } from '@/lib/types';
import { MessageBubble } from './MessageBubble';
import { HealthStatus } from './HealthStatus';

export function ChatInterface() {
  const [messages, setMessages] = useState<UIMessage[]>([]);
  const [input, setInput] = useState('');
  const [useRAG, setUseRAG] = useState(true);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // Auto-resize textarea
  useEffect(() => {
    const el = textareaRef.current;
    if (el) {
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
    }
  }, [input]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const text = input.trim();
    if (!text || isLoading) return;

    setInput('');
    setError(null);

    // Add user message immediately
    const userMsg: UIMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: text,
      timestamp: Date.now(),
    };

    // Add placeholder loading message
    const loadingMsg: UIMessage = {
      id: `loading-${Date.now()}`,
      role: 'assistant',
      content: '',
      timestamp: Date.now(),
      isLoading: true,
    };

    setMessages((prev) => [...prev, userMsg, loadingMsg]);
    setIsLoading(true);

    try {
      // Create conversation on first message
      let convId = conversationId;
      if (!convId) {
        convId = await createConversation();
        setConversationId(convId);
      }

      const response = await sendChat({
        message: text,
        conversation_id: convId,
        use_rag: useRAG,
      });

      const assistantMsg: UIMessage = {
        id: response.id,
        role: 'assistant',
        content: response.content,
        sources: response.sources,
        timestamp: response.timestamp * 1000,
      };

      // Replace loading message with real response
      setMessages((prev) => [...prev.slice(0, -1), assistantMsg]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send message');
      // Remove loading placeholder
      setMessages((prev) => prev.slice(0, -1));
    } finally {
      setIsLoading(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(e as unknown as React.FormEvent);
    }
  }

  function startNewConversation() {
    setMessages([]);
    setConversationId(null);
    setError(null);
  }

  return (
    <div className="flex h-screen flex-col bg-slate-900 text-slate-100">
      {/* Header */}
      <header className="flex items-center justify-between border-b border-slate-700 bg-slate-900/95 px-6 py-3 backdrop-blur">
        <div className="flex items-center gap-3">
          <span className="text-2xl">🍽️</span>
          <div>
            <h1 className="text-lg font-semibold text-emerald-400">
              Food Nutrition AI
            </h1>
            {conversationId && (
              <p className="text-xs text-slate-500 font-mono">
                {conversationId}
              </p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-4">
          <HealthStatus />
          {messages.length > 0 && (
            <button
              onClick={startNewConversation}
              className="rounded-lg border border-slate-700 px-3 py-1.5 text-sm text-slate-400 hover:border-slate-500 hover:text-slate-200 transition-colors"
            >
              New chat
            </button>
          )}
        </div>
      </header>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-4 py-6">
        {messages.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
            <span className="text-5xl">🥗</span>
            <div>
              <p className="text-xl font-semibold text-slate-200">
                Food Nutrition AI
              </p>
              <p className="mt-1 text-slate-500">
                Ask about calories, macros, ingredients, and more.
              </p>
            </div>
            <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2 text-sm max-w-lg w-full">
              {[
                'What are the nutrients in Coca-Cola?',
                'Compare protein in chicken vs tofu',
                'High fiber breakfast options',
                'Calories in a McDonald\'s Big Mac',
              ].map((suggestion) => (
                <button
                  key={suggestion}
                  onClick={() => setInput(suggestion)}
                  className="rounded-xl border border-slate-700 bg-slate-800 px-4 py-3 text-left text-slate-300 hover:border-emerald-700 hover:bg-slate-700 transition-colors"
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="mx-auto max-w-3xl space-y-6">
            {messages.map((msg) => (
              <MessageBubble key={msg.id} message={msg} />
            ))}
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Error banner */}
      {error && (
        <div className="mx-4 mb-2 rounded-lg border border-red-800 bg-red-900/30 px-4 py-2 text-sm text-red-300">
          {error}
          <button
            onClick={() => setError(null)}
            className="ml-2 text-red-400 hover:text-red-200"
          >
            ✕
          </button>
        </div>
      )}

      {/* Input area */}
      <div className="border-t border-slate-700 bg-slate-900/95 px-4 py-4 backdrop-blur">
        <form
          onSubmit={handleSubmit}
          className="mx-auto flex max-w-3xl flex-col gap-2"
        >
          <div className="flex items-end gap-2 rounded-2xl border border-slate-700 bg-slate-800 px-4 py-3 focus-within:border-emerald-600 transition-colors">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Ask about any food…"
              rows={1}
              disabled={isLoading}
              className="flex-1 resize-none bg-transparent text-sm text-slate-100 placeholder-slate-500 outline-none disabled:opacity-50"
            />
            <button
              type="submit"
              disabled={!input.trim() || isLoading}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              aria-label="Send"
            >
              <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
                <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" />
              </svg>
            </button>
          </div>
          <div className="flex items-center justify-between px-1">
            <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-400 select-none">
              <div
                onClick={() => setUseRAG((v) => !v)}
                className={`relative h-5 w-9 rounded-full transition-colors ${useRAG ? 'bg-emerald-600' : 'bg-slate-600'}`}
              >
                <span
                  className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${useRAG ? 'translate-x-4' : 'translate-x-0.5'}`}
                />
              </div>
              RAG (food database search)
            </label>
            <p className="text-xs text-slate-600">
              Enter to send · Shift+Enter for new line
            </p>
          </div>
        </form>
      </div>
    </div>
  );
}
