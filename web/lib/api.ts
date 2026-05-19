import {
  ChatRequest,
  ChatResponse,
  Conversation,
  RAGSearchResponse,
} from './types';

const BASE = '/api';

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export async function checkHealth(): Promise<boolean> {
  try {
    await request<{ status: string }>('/health');
    return true;
  } catch {
    return false;
  }
}

export async function createConversation(): Promise<string> {
  const data = await request<{ id: string }>('/conversations', {
    method: 'POST',
    body: JSON.stringify({}),
  });
  return data.id;
}

export async function getConversation(id: string): Promise<Conversation> {
  return request<Conversation>(`/conversations/${id}`);
}

export async function sendChat(req: ChatRequest): Promise<ChatResponse> {
  return request<ChatResponse>('/chat', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function ragSearch(query: string): Promise<RAGSearchResponse> {
  return request<RAGSearchResponse>('/rag/search', {
    method: 'POST',
    body: JSON.stringify({ query }),
  });
}
