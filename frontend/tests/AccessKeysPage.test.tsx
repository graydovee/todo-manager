import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AccessKeysPage } from '../src/pages/AccessKeysPage'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'accessKeys.title': 'Access Keys',
        'accessKeys.subtitle': 'Manage API keys for CLI and AI clients.',
        'accessKeys.create': 'Create Key',
        'accessKeys.createTitle': 'Create Access Key',
        'accessKeys.name': 'Name',
        'accessKeys.nameRequired': 'Please enter a name',
        'accessKeys.expiresAt': 'Expires At',
        'accessKeys.noticeTitle': 'Store the key safely',
        'accessKeys.noticeBody': 'The full key is shown only once after create or rotate. After closing the dialog, only the prefix remains visible.',
        'accessKeys.presets.read': 'Read',
        'accessKeys.presets.write': 'Write',
        'accessKeys.presets.readWrite': 'Read + Write',
        'accessKeys.presets.summary': 'Summary',
        'accessKeys.close': 'Close',
      }
      return translations[key] || key
    },
  }),
}))

const listAccessKeys = vi.fn()
const getAccessKeyPermissionCatalog = vi.fn()
const createAccessKey = vi.fn()

vi.mock('../src/api/accessKeys', () => ({
  listAccessKeys: (...args: unknown[]) => listAccessKeys(...args),
  getAccessKeyPermissionCatalog: (...args: unknown[]) => getAccessKeyPermissionCatalog(...args),
  createAccessKey: (...args: unknown[]) => createAccessKey(...args),
  rotateAccessKey: vi.fn(),
  deleteAccessKey: vi.fn(),
}))

describe('AccessKeysPage', () => {
  beforeEach(() => {
    listAccessKeys.mockResolvedValue([])
    getAccessKeyPermissionCatalog.mockResolvedValue({
      apis: [
        { id: 'todos:list', group_id: 'todos_read', method: 'GET', path_pattern: '/api/v1/todos', label: 'List todos', description: 'Read the todo list.' },
        { id: 'todos:create', group_id: 'todos_write', method: 'POST', path_pattern: '/api/v1/todos', label: 'Create todo', description: 'Create a todo.' },
      ],
      groups: [
        { id: 'todos_read', label: 'Read APIs', description: 'Read-only todo APIs.', permission_ids: ['todos:list'] },
        { id: 'todos_write', label: 'Write APIs', description: 'Todo mutation APIs.', permission_ids: ['todos:create'] },
      ],
      presets: {
        read: ['todos:list'],
        write: ['todos:create'],
        read_write: ['todos:list', 'todos:create'],
        summary: [],
      },
    })
    createAccessKey.mockResolvedValue({
      id: 1,
      name: 'CLI',
      key_prefix: 'tdk_abc123def456',
      authorized_apis: ['todos:list'],
      expires_at: null,
      last_used_at: null,
      updated_at: '2026-06-15T00:00:00Z',
      created_at: '2026-06-15T00:00:00Z',
      plain_key: 'tdk_abc123def456_secret',
    })
  })

  it('applies read preset and submits concrete api list', async () => {
    render(<AccessKeysPage />)

    await waitFor(() => {
      expect(screen.getByText('Create Key')).toBeTruthy()
    })

    fireEvent.click(screen.getByText('Create Key'))
    fireEvent.change(document.getElementById('name') as HTMLInputElement, { target: { value: 'CLI' } })
    fireEvent.click(screen.getByText('Read'))
    fireEvent.click(screen.getByText('OK'))

    await waitFor(() => {
      expect(createAccessKey).toHaveBeenCalledWith({
        name: 'CLI',
        authorized_apis: ['todos:list'],
        expires_at: undefined,
      })
    })
  })
})
