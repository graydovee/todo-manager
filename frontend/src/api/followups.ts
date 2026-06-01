import { client, csrfToken } from './client';
import type { FollowupMessage, ContextMessage } from '../types/followup';

/**
 * Create a followup question for a summary via POST request.
 * Returns the fetch Response object for streaming SSE data.
 * Uses fetch API instead of EventSource because the endpoint requires POST.
 */
export async function createFollowup(
  summaryId: number,
  question: string,
  contextMessages?: ContextMessage[]
): Promise<Response> {
  const body: Record<string, unknown> = { question };
  if (contextMessages && contextMessages.length > 0) {
    body.context_messages = contextMessages;
  }

  const response = await fetch(`/api/v1/summaries/${summaryId}/followup`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
    },
    credentials: 'include',
    body: JSON.stringify(body),
  });

  return response;
}

/**
 * List all followup messages (with versions) for a given summary.
 * Returns messages ordered by created_at ascending, versions by version_number ascending.
 */
export async function listFollowups(summaryId: number): Promise<FollowupMessage[]> {
  const res = await client.get(`/summaries/${summaryId}/followups`);
  return res.data;
}

/**
 * Helper that returns the followup stream URL for a given summary.
 * Useful for constructing the endpoint path in components.
 */
export function getFollowupStreamUrl(
  summaryId: number,
  question: string,
  contextMessages?: ContextMessage[]
): string {
  const params = new URLSearchParams();
  params.set('question', question);
  if (contextMessages && contextMessages.length > 0) {
    params.set('context_messages', JSON.stringify(contextMessages));
  }
  return `/api/v1/summaries/${summaryId}/followup?${params.toString()}`;
}
