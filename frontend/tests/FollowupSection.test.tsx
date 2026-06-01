import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react'

// Polyfill ResizeObserver for jsdom
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('ResizeObserver', MockResizeObserver)

// Polyfill scrollIntoView for jsdom
Element.prototype.scrollIntoView = vi.fn()

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'aiSummary.followup.inputPlaceholder': 'Ask a follow-up question...',
        'aiSummary.followup.send': 'Send',
        'aiSummary.followup.loading': 'Loading...',
        'aiSummary.followup.error': 'An error occurred',
        'aiSummary.followup.networkError': 'Network error',
        'aiSummary.followup.editTooltip': 'Edit',
        'aiSummary.followup.regenerateTooltip': 'Regenerate',
        'aiSummary.followup.retryButton': 'Retry',
        'aiSummary.followup.versionIndicator': '{{current}}/{{total}}',
      }
      return translations[key] || key
    },
  }),
}))

// Mock the followups API module
vi.mock('../src/api/followups', () => ({
  listFollowups: vi.fn(),
  createFollowup: vi.fn(),
}))

import { listFollowups, createFollowup } from '../src/api/followups'
import { FollowupSection } from '../src/components/FollowupSection'

const mockedListFollowups = vi.mocked(listFollowups)
const mockedCreateFollowup = vi.mocked(createFollowup)

beforeEach(() => {
  vi.clearAllMocks()
  // Default: no existing messages
  mockedListFollowups.mockResolvedValue([])
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('FollowupSection', () => {
  describe('Rendering', () => {
    it('renders the followup section with input area and send button', async () => {
      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      await waitFor(() => {
        expect(screen.getByTestId('followup-section')).toBeDefined()
      })

      // Input area should be present
      expect(screen.getByTestId('followup-input')).toBeDefined()
      // Send button should be present
      expect(screen.getByTestId('followup-send-btn')).toBeDefined()
    })
  })

  describe('Send button state', () => {
    it('send button is disabled when input is empty', async () => {
      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      await waitFor(() => {
        expect(screen.getByTestId('followup-send-btn')).toBeDefined()
      })

      const sendBtn = screen.getByTestId('followup-send-btn')
      expect(sendBtn.closest('button')?.disabled).toBe(true)
    })

    it('send button is enabled when input has text', async () => {
      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      await waitFor(() => {
        expect(screen.getByTestId('followup-input')).toBeDefined()
      })

      const input = screen.getByTestId('followup-input')
      await act(async () => {
        fireEvent.change(input, { target: { value: 'Hello' } })
      })

      const sendBtn = screen.getByTestId('followup-send-btn')
      expect(sendBtn.closest('button')?.disabled).toBe(false)
    })
  })

  describe('Loading history', () => {
    it('displays loading indicator while loading history', async () => {
      // Make listFollowups hang to observe loading state
      let resolveList: (value: unknown[]) => void
      mockedListFollowups.mockImplementation(
        () => new Promise((resolve) => { resolveList = resolve as (value: unknown[]) => void })
      )

      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      // Loading indicator should be visible
      await waitFor(() => {
        const section = screen.getByTestId('followup-section')
        expect(section.textContent).toContain('Loading...')
      })

      // Resolve the promise to clean up
      await act(async () => {
        resolveList!([])
      })
    })

    it('loads and displays existing followup messages on mount', async () => {
      mockedListFollowups.mockResolvedValue([
        {
          id: 1,
          summary_id: 1,
          question: 'What about bugs?',
          versions: [
            { id: 1, content: 'Here is the bug analysis.', version_number: 1, created_at: '2024-01-15T10:30:00Z' },
          ],
          created_at: '2024-01-15T10:30:00Z',
        },
        {
          id: 2,
          summary_id: 1,
          question: 'Can you elaborate?',
          versions: [
            { id: 2, content: 'Sure, here are more details.', version_number: 1, created_at: '2024-01-15T10:31:00Z' },
          ],
          created_at: '2024-01-15T10:31:00Z',
        },
      ])

      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      // Wait for messages to load and render
      await waitFor(() => {
        const bubbles = screen.getAllByTestId('followup-message-bubble')
        expect(bubbles.length).toBe(2)
      })

      // Verify the questions are displayed
      expect(screen.getByText('What about bugs?')).toBeDefined()
      expect(screen.getByText('Can you elaborate?')).toBeDefined()
    })
  })

  describe('Chronological order', () => {
    it('messages are displayed in chronological order', async () => {
      mockedListFollowups.mockResolvedValue([
        {
          id: 1,
          summary_id: 1,
          question: 'First question',
          versions: [
            { id: 1, content: 'First answer', version_number: 1, created_at: '2024-01-15T10:00:00Z' },
          ],
          created_at: '2024-01-15T10:00:00Z',
        },
        {
          id: 2,
          summary_id: 1,
          question: 'Second question',
          versions: [
            { id: 2, content: 'Second answer', version_number: 1, created_at: '2024-01-15T11:00:00Z' },
          ],
          created_at: '2024-01-15T11:00:00Z',
        },
        {
          id: 3,
          summary_id: 1,
          question: 'Third question',
          versions: [
            { id: 3, content: 'Third answer', version_number: 1, created_at: '2024-01-15T12:00:00Z' },
          ],
          created_at: '2024-01-15T12:00:00Z',
        },
      ])

      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      await waitFor(() => {
        const bubbles = screen.getAllByTestId('followup-message-bubble')
        expect(bubbles.length).toBe(3)
      })

      // Verify order: first question appears before second, second before third
      const section = screen.getByTestId('followup-section')
      const text = section.textContent || ''
      const firstIdx = text.indexOf('First question')
      const secondIdx = text.indexOf('Second question')
      const thirdIdx = text.indexOf('Third question')

      expect(firstIdx).toBeLessThan(secondIdx)
      expect(secondIdx).toBeLessThan(thirdIdx)
    })
  })

  describe('Version navigation UI', () => {
    it('does not show version navigation for single-version messages', async () => {
      mockedListFollowups.mockResolvedValue([
        {
          id: 1,
          summary_id: 1,
          question: 'A question',
          versions: [
            { id: 1, content: 'Single version answer', version_number: 1, created_at: '2024-01-15T10:30:00Z' },
          ],
          created_at: '2024-01-15T10:30:00Z',
        },
      ])

      render(<FollowupSection summaryId={1} summaryContent="Test summary" />)

      await waitFor(() => {
        expect(screen.getAllByTestId('followup-message-bubble').length).toBe(1)
      })

      // Version navigation should not be present for single-version messages
      expect(screen.queryByTestId('followup-version-nav')).toBeNull()
    })
  })
})
