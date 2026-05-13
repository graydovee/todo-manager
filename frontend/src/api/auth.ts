import { client } from './client';
import type { AuthMode, User } from '../types';

export async function getAuthMode(): Promise<AuthMode> {
  const res = await client.get('/auth/mode');
  return res.data;
}

export async function getMe(): Promise<User> {
  const res = await client.get('/auth/me');
  return res.data;
}

export async function login(username: string, password: string): Promise<User> {
  const res = await client.post('/auth/login', { username, password });
  return res.data;
}

export async function logout(): Promise<void> {
  await client.post('/auth/logout');
}
