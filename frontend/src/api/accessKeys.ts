import { client } from './client';
import type {
  AccessKey,
  AccessKeyPermissionCatalog,
  CreateAccessKeyInput,
  RotateAccessKeyResponse,
} from '../types';

export async function listAccessKeys(): Promise<AccessKey[]> {
  const res = await client.get('/access-keys');
  return res.data;
}

export async function getAccessKeyPermissionCatalog(): Promise<AccessKeyPermissionCatalog> {
  const res = await client.get('/access-keys/permissions');
  return res.data;
}

export async function createAccessKey(input: CreateAccessKeyInput): Promise<RotateAccessKeyResponse> {
  const res = await client.post('/access-keys', input);
  return res.data;
}

export async function rotateAccessKey(id: number): Promise<RotateAccessKeyResponse> {
  const res = await client.post(`/access-keys/${id}/rotate`);
  return res.data;
}

export async function deleteAccessKey(id: number): Promise<void> {
  await client.delete(`/access-keys/${id}`);
}
