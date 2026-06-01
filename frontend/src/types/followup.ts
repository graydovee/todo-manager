export interface FollowupMessage {
  id: number;
  summary_id: number;
  question: string;
  versions: MessageVersion[];
  created_at: string;
}

export interface MessageVersion {
  id: number;
  content: string;
  version_number: number;
  created_at: string;
}

export interface ContextMessage {
  role: 'user' | 'assistant';
  content: string;
}

export interface FollowupRequest {
  question: string;
  context_messages?: ContextMessage[];
}
