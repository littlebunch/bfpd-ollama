export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

export interface RAGSource {
  id: string;
  content: string;
  metadata: Record<string, unknown>;
  score: number;
}

export interface ChatRequest {
  message: string;
  conversation_id?: string;
  use_rag?: boolean;
}

export interface ChatResponse {
  id: string;
  conversation_id: string;
  role: string;
  content: string;
  sources?: RAGSource[];
  timestamp: number;
}

export interface Conversation {
  ID: string;
  Messages: ChatMessage[];
  Created: number;
  Updated: number;
}

export interface RAGSearchRequest {
  query: string;
}

export interface RAGSearchResponse {
  sources: RAGSource[];
}

/** UI-only model combining chat response data with display state */
export interface UIMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  sources?: RAGSource[];
  timestamp: number;
  isLoading?: boolean;
}
