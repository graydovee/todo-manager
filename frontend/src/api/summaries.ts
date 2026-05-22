import { client } from './client';

export interface SummaryEntry {
  id: number;
  start_date: string;
  end_date: string;
  status: 'analyzing' | 'completed' | 'error';
  result_content?: string;
  created_at: string;
  updated_at: string;
}

export async function createSummary(startDate: string, endDate: string): Promise<SummaryEntry> {
  const res = await client.post('/summaries', { start_date: startDate, end_date: endDate });
  return res.data;
}

export async function listSummaries(): Promise<SummaryEntry[]> {
  const res = await client.get('/summaries');
  return res.data;
}

export async function getSummary(id: number): Promise<SummaryEntry> {
  const res = await client.get(`/summaries/${id}`);
  return res.data;
}

export async function deleteSummary(id: number): Promise<void> {
  await client.delete(`/summaries/${id}`);
}
