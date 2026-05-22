import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, waitFor, cleanup } from '@testing-library/react'
import { SummaryDetailPanel } from '../src/components/SummaryDetailPanel'

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'summaryDetail.selectSummary': 'Select a summary from the left to view details',
        'summaryDetail.emptyContent': 'No analysis content available',
        'analysis.result.error': 'Analysis Error',
        'analysis.result.streaming': 'AI is generating analysis...',
        'analysis.result.retryButton': 'Retry',
        'analysis.result.connectionError': 'Connection Lost',
        'analysis.result.connectionErrorDesc': 'Connection to server was lost',
      }
      return translations[key] || key
    },
  }),
}))

// Mock the summaries API module
vi.mock('../src/api/summaries', () => ({
  getSummary: vi.fn(),
  getStreamUrl: vi.fn((id: number) => `/api/v1/summaries/${id}/stream`),
}))

// Import the mocked module for controlling return values
import { getSummary } from '../src/api/summaries'
const mockedGetSummary = vi.mocked(getSummary)

// Mock EventSource
class MockEventSource {
  static instances: MockEventSource[] = []
  url: string
  withCredentials: boolean
  readyState: number = 0
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  private listeners: Record<string, EventListener[]> = {}

  constructor(url: string, options?: { withCredentials?: boolean }) {
    this.url = url
    this.withCredentials = options?.withCredentials ?? false
    MockEventSource.instances.push(this)
  }

  addEventListener(type: string, listener: EventListener) {
    if (!this.listeners[type]) {
      this.listeners[type] = []
    }
    this.listeners[type].push(listener)
  }

  removeEventListener(type: string, listener: EventListener) {
    if (this.listeners[type]) {
      this.listeners[type] = this.listeners[type].filter((l) => l !== listener)
    }
  }

  close() {
    this.readyState = 2
  }

  // Test helpers
  simulateMessage(data: string) {
    const event = new MessageEvent('message', { data })
    if (this.onmessage) {
      this.onmessage(event)
    }
  }

  simulateDone() {
    const event = new MessageEvent('done', { data: '' })
    if (this.listeners['done']) {
      this.listeners['done'].forEach((l) => l(event as unknown as Event))
    }
  }

  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 2
}

beforeEach(() => {
  MockEventSource.instances = []
  vi.stubGlobal('EventSource', MockEventSource)
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('SummaryDetailPanel', () => {
  describe('Placeholder state (summaryId=null)', () => {
    it('displays placeholder text when no summary is selected', () => {
      const { container } = render(<SummaryDetailPanel summaryId={null} />)
      const placeholder = container.querySelector('.summary-detail-panel__placeholder')
      expect(placeholder).not.toBeNull()
      expect(placeholder!.textContent).toBe('Select a summary from the left to view details')
    })

    it('does not show loading, error, or content areas', () => {
      const { container } = render(<SummaryDetailPanel summaryId={null} />)
      expect(container.querySelector('.summary-detail-panel__loading')).toBeNull()
      expect(container.querySelector('.summary-detail-panel__error')).toBeNull()
      expect(container.querySelector('.summary-detail-panel__content')).toBeNull()
    })
  })

  describe('Completed summary - Markdown rendering', () => {
    it('renders Markdown content for a completed summary', async () => {
      mockedGetSummary.mockResolvedValue({
        id: 1,
        start_date: '2024-01-01',
        end_date: '2024-01-07',
        status: 'completed',
        result_content: '## Summary Title\n\nThis is a **bold** paragraph.',
        created_at: '2024-01-07T10:00:00Z',
        updated_at: '2024-01-07T10:00:00Z',
      })

      const { container } = render(<SummaryDetailPanel summaryId={1} />)

      await waitFor(() => {
        const heading = container.querySelector('h2')
        expect(heading).not.toBeNull()
        expect(heading!.textContent).toBe('Summary Title')
      })

      // Verify bold text is rendered as <strong>
      const strong = container.querySelector('strong')
      expect(strong).not.toBeNull()
      expect(strong!.textContent).toBe('bold')
    })

    it('shows empty content message when result_content is empty', async () => {
      mockedGetSummary.mockResolvedValue({
        id: 2,
        start_date: '2024-01-01',
        end_date: '2024-01-07',
        status: 'completed',
        result_content: '',
        created_at: '2024-01-07T10:00:00Z',
        updated_at: '2024-01-07T10:00:00Z',
      })

      const { container } = render(<SummaryDetailPanel summaryId={2} />)

      await waitFor(() => {
        const placeholder = container.querySelector('.summary-detail-panel__placeholder')
        expect(placeholder).not.toBeNull()
        expect(placeholder!.textContent).toBe('No analysis content available')
      })
    })
  })

  describe('Streaming indicator during SSE', () => {
    it('shows streaming hint when summary is analyzing and no content yet', async () => {
      mockedGetSummary.mockResolvedValue({
        id: 3,
        start_date: '2024-01-01',
        end_date: '2024-01-07',
        status: 'analyzing',
        result_content: '',
        created_at: '2024-01-07T10:00:00Z',
        updated_at: '2024-01-07T10:00:00Z',
      })

      const { container } = render(<SummaryDetailPanel summaryId={3} />)

      // Wait for the API call to resolve and SSE to start
      await waitFor(() => {
        expect(MockEventSource.instances.length).toBeGreaterThan(0)
      })

      // The streaming hint should be visible (no content yet)
      await waitFor(() => {
        const hint = container.querySelector('.summary-detail-panel__streaming-hint')
        expect(hint).not.toBeNull()
        expect(hint!.textContent).toContain('AI is generating analysis...')
      })
    })

    it('shows blinking cursor when streaming content arrives', async () => {
      mockedGetSummary.mockResolvedValue({
        id: 4,
        start_date: '2024-01-01',
        end_date: '2024-01-07',
        status: 'analyzing',
        result_content: '',
        created_at: '2024-01-07T10:00:00Z',
        updated_at: '2024-01-07T10:00:00Z',
      })

      const { container } = render(<SummaryDetailPanel summaryId={4} />)

      // Wait for EventSource to be created
      await waitFor(() => {
        expect(MockEventSource.instances.length).toBeGreaterThan(0)
      })

      const es = MockEventSource.instances[MockEventSource.instances.length - 1]

      // Simulate receiving content via onmessage
      es.simulateMessage('## Hello World')

      // Wait for React to process the state update
      await waitFor(() => {
        const cursor = container.querySelector('.summary-detail-panel__cursor')
        expect(cursor).not.toBeNull()
      })
    })

    it('removes streaming cursor after done event', async () => {
      mockedGetSummary.mockResolvedValue({
        id: 5,
        start_date: '2024-01-01',
        end_date: '2024-01-07',
        status: 'analyzing',
        result_content: '',
        created_at: '2024-01-07T10:00:00Z',
        updated_at: '2024-01-07T10:00:00Z',
      })

      const { container } = render(<SummaryDetailPanel summaryId={5} />)

      // Wait for streaming hint to appear (SSE connected, no content yet)
      await waitFor(() => {
        const hint = container.querySelector('.summary-detail-panel__streaming-hint')
        expect(hint).not.toBeNull()
      })

      const es = MockEventSource.instances[MockEventSource.instances.length - 1]

      // Simulate receiving content
      es.simulateMessage('Some content')

      // Verify cursor is present
      await waitFor(() => {
        expect(container.querySelector('.summary-detail-panel__cursor')).not.toBeNull()
      })

      // Simulate done event
      es.simulateDone()

      // Cursor should be removed after done
      await waitFor(() => {
        expect(container.querySelector('.summary-detail-panel__cursor')).toBeNull()
      })
    })
  })

  describe('Error state display', () => {
    it('displays error message for summary with error status', async () => {
      mockedGetSummary.mockResolvedValue({
        id: 6,
        start_date: '2024-01-01',
        end_date: '2024-01-07',
        status: 'error',
        result_content: 'LLM service unavailable',
        created_at: '2024-01-07T10:00:00Z',
        updated_at: '2024-01-07T10:00:00Z',
      })

      const { container } = render(<SummaryDetailPanel summaryId={6} />)

      await waitFor(() => {
        const errorTitle = container.querySelector('.summary-detail-panel__error-title')
        expect(errorTitle).not.toBeNull()
      })

      const errorTitle = container.querySelector('.summary-detail-panel__error-title')
      expect(errorTitle!.textContent).toBe('Analysis Error')

      const errorDesc = container.querySelector('.summary-detail-panel__error-desc')
      expect(errorDesc).not.toBeNull()
      expect(errorDesc!.textContent).toBe('LLM service unavailable')
    })

    it('displays generic error when API call fails', async () => {
      mockedGetSummary.mockRejectedValue(new Error('Network error'))

      const { container } = render(<SummaryDetailPanel summaryId={7} />)

      await waitFor(() => {
        const errorTitle = container.querySelector('.summary-detail-panel__error-title')
        expect(errorTitle).not.toBeNull()
      })

      const errorTitle = container.querySelector('.summary-detail-panel__error-title')
      expect(errorTitle!.textContent).toBe('Analysis Error')
    })
  })
})
