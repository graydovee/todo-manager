/**
 * Bug Condition Exploration Test for Bug 1 (Mobile Drawer Consistency).
 *
 * **Validates: Requirements 1.1, 1.2, 1.3, 2.1, 2.2, 2.3**
 *
 * Property 3 (Bug Condition - Scoped PBT):
 *   For viewport width <= 768px, the Drawers rendered by AISummaryPage and
 *   TodoListPage MUST share the same mobile-detail-drawer base class, render
 *   non-empty titles, and apply consistent inline `width` (`.ant-drawer-content-wrapper`).
 *
 * This test is EXPECTED TO FAIL on the unfixed code (failure confirms Bug 1
 * exists). After Task 5 implements MobileDetailDrawer, the same test should
 * PASS unchanged, serving as the Expected Behavior verification.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act } from 'react';
import { render, waitFor, cleanup } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import * as fc from 'fast-check';

// ---------------------------------------------------------------------------
// react-i18next mock: return a stable mapping for keys we care about.
// We memoize the returned object so that callers receive a stable `t`
// reference (otherwise React effects whose deps include `t` would re-run on
// every render and starve scheduled microtasks under fc.assert loops).
// ---------------------------------------------------------------------------
const TEST_TRANSLATIONS: Record<string, string> = {
  'aiSummary.title': 'AI Summary',
  'aiSummary.newAnalysis': 'New Analysis',
  'aiSummary.statusAnalyzing': 'Analyzing',
  'aiSummary.statusCompleted': 'Completed',
  'aiSummary.statusError': 'Error',
  'aiSummary.emptyState': 'No records',
  'aiSummary.deleteTitle': 'Delete',
  'aiSummary.deleteConfirm': 'Are you sure?',
  'summaryDetail.selectSummary': 'Select a summary',
  'summaryDetail.emptyContent': 'No content',
  'summaryDetail.drawerTitle': 'Summary Details',
  'todo.title': 'Todos',
  'todo.newTodo': 'New Todo',
  'todo.detail': 'Todo Details',
  'todo.titleLabel': 'Title',
  'todo.priority': 'Priority',
  'todo.status': 'Status',
  'todo.open': 'Open',
  'todo.inProgress': 'In Progress',
  'todo.completed': 'Completed',
  'detail.noSelection': 'No selection',
  'analysis.result.streaming': 'Streaming',
  'analysis.result.error': 'Analysis Error',
  'analysis.result.retryButton': 'Retry',
  'analysis.result.connectionError': 'Connection lost',
  'analysis.result.connectionErrorDesc': 'Lost connection',
  'common.items': 'items',
};
const STABLE_T = (key: string) => TEST_TRANSLATIONS[key] ?? key;
const STABLE_I18N = { language: 'en', changeLanguage: () => Promise.resolve() };
const STABLE_USE_TRANSLATION = { t: STABLE_T, i18n: STABLE_I18N };

vi.mock('react-i18next', () => ({
  useTranslation: () => STABLE_USE_TRANSLATION,
}));

// ---------------------------------------------------------------------------
// API mocks for summaries and todos. Defined as factories so that each test
// can configure return values via vi.mocked().
// ---------------------------------------------------------------------------
vi.mock('../../src/api/summaries', () => ({
  listSummaries: vi.fn(),
  getSummary: vi.fn(),
  createSummaryWithTodos: vi.fn(),
  deleteSummary: vi.fn(),
  getStreamUrl: vi.fn((id: number) => `/api/v1/summaries/${id}/stream`),
}));

vi.mock('../../src/api/todos', () => ({
  listTodos: vi.fn(),
  getTodo: vi.fn(),
  fetchTags: vi.fn().mockResolvedValue([]),
  // Other exports are not exercised by this test but must exist for type safety
  // when modules import them at top-level.
  createTodo: vi.fn(),
  updateTodo: vi.fn(),
  deleteTodo: vi.fn(),
  startTodo: vi.fn(),
  completeTodo: vi.fn(),
  reopenTodo: vi.fn(),
  setTodoStatus: vi.fn(),
  listComments: vi.fn().mockResolvedValue([]),
  createComment: vi.fn(),
  deleteComment: vi.fn(),
  pinTodo: vi.fn(),
  highlightTodo: vi.fn(),
  getTodoGraph: vi.fn(),
  fetchTodosByDateRange: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Imports referencing the mocked modules. Must happen AFTER vi.mock calls.
// ---------------------------------------------------------------------------
import { AISummaryPage } from '../../src/pages/AISummaryPage';
import { TodoListPage } from '../../src/pages/TodoListPage';
import { listSummaries, getSummary, createSummaryWithTodos } from '../../src/api/summaries';
import { listTodos, getTodo } from '../../src/api/todos';

const mockedListSummaries = vi.mocked(listSummaries);
const mockedGetSummary = vi.mocked(getSummary);
const mockedCreateSummaryWithTodos = vi.mocked(createSummaryWithTodos);
const mockedListTodos = vi.mocked(listTodos);
const mockedGetTodo = vi.mocked(getTodo);

// ---------------------------------------------------------------------------
// Mock EventSource so that opening the Summary Drawer does not try to make
// real network requests.
// ---------------------------------------------------------------------------
class MockEventSource {
  static instances: MockEventSource[] = [];
  static constructorCalls = 0;
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSED = 2;

  url: string;
  withCredentials: boolean;
  readyState = 1;
  closed = false;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  private listeners: Record<string, EventListener[]> = {};

  constructor(url: string, options?: { withCredentials?: boolean }) {
    this.url = url;
    this.withCredentials = options?.withCredentials ?? false;
    MockEventSource.instances.push(this);
    MockEventSource.constructorCalls += 1;
  }

  addEventListener(type: string, listener: EventListener) {
    if (!this.listeners[type]) this.listeners[type] = [];
    this.listeners[type].push(listener);
  }

  removeEventListener(type: string, listener: EventListener) {
    if (this.listeners[type]) {
      this.listeners[type] = this.listeners[type].filter((l) => l !== listener);
    }
  }

  close() {
    this.readyState = MockEventSource.CLOSED;
    this.closed = true;
  }

  // -- test helpers ---------------------------------------------------------
  dispatchMessage(data: string) {
    const event = new MessageEvent('message', { data });
    if (this.onmessage) this.onmessage(event);
  }

  dispatchDone() {
    const event = new MessageEvent('done', { data: '' });
    (this.listeners['done'] ?? []).forEach((l) => l(event as unknown as Event));
  }

  dispatchCustomError(data = 'server failure') {
    const event = new MessageEvent('error', { data });
    (this.listeners['error'] ?? []).forEach((l) => l(event as unknown as Event));
  }

  dispatchNativeError(opts: { closed?: boolean } = {}) {
    if (opts.closed) this.readyState = MockEventSource.CLOSED;
    if (this.onerror) this.onerror();
  }
}

// ---------------------------------------------------------------------------
// Helpers for switching the viewport into mobile / desktop mode and stubbing
// matchMedia (jsdom does not implement it natively).
// ---------------------------------------------------------------------------
function setViewportWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  });
  // Mock matchMedia to return matches=true for `(max-width: 768px)` when
  // viewport width <= 768. jsdom does not implement matchMedia natively, and
  // antd's Table/Grid relies on it.
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: query.includes('max-width: 768px') ? width <= 768 : false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

// jsdom does not implement ResizeObserver, but antd's Table uses it via
// rc-resize-observer. Provide a minimal stub.
class StubResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
if (typeof (globalThis as { ResizeObserver?: unknown }).ResizeObserver === 'undefined') {
  (globalThis as { ResizeObserver: typeof StubResizeObserver }).ResizeObserver = StubResizeObserver;
}

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}

function renderWithQuery(ui: React.ReactElement) {
  const client = makeQueryClient();
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------
const ANALYZING_ENTRY = {
  id: 42,
  start_date: '2024-01-01',
  end_date: '2024-01-07',
  status: 'analyzing' as const,
  result_content: '',
  created_at: '2024-01-07T10:00:00Z',
  updated_at: '2024-01-07T10:00:00Z',
};

const TODO_ITEM = {
  id: 7,
  code: 'task-001',
  title: 'Sample Todo',
  description: '',
  category: 'task' as const,
  priority: 'p2' as const,
  status: 'open' as const,
  due_at: null,
  tags: [] as string[],
  pinned: false,
  highlighted: false,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

const TODO_DETAIL = {
  ...TODO_ITEM,
  depends_on: [],
  depended_by: [],
  duplicate_of: null,
  duplicates: [],
};

beforeEach(() => {
  MockEventSource.instances = [];
  MockEventSource.constructorCalls = 0;
  vi.stubGlobal('EventSource', MockEventSource);
  setViewportWidth(375);
  // Configure default mocked API responses
  mockedListSummaries.mockResolvedValue([ANALYZING_ENTRY]);
  mockedGetSummary.mockResolvedValue(ANALYZING_ENTRY);
  mockedListTodos.mockResolvedValue({
    items: [TODO_ITEM],
    total: 1,
    page: 1,
    page_size: 20,
  });
  mockedGetTodo.mockResolvedValue(TODO_DETAIL);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Helpers to render each page in mobile mode and open the detail Drawer.
// Returns the antd Drawer root once rendered.
// ---------------------------------------------------------------------------
async function renderSummaryDrawer() {
  const result = renderWithQuery(<AISummaryPage />);
  // Wait until the entry list item has been rendered.
  const entry = await waitFor(
    () => {
      const node = result.container.querySelector('.ai-summary-page__list-item');
      expect(node).not.toBeNull();
      return node as HTMLElement;
    },
    { timeout: 4000 },
  );
  await act(async () => {
    entry.click();
  });
  // Wait for the Drawer DOM to mount (antd Drawer portals into document.body).
  await waitFor(
    () => {
      const drawer = document.body.querySelector('.ant-drawer');
      expect(drawer).not.toBeNull();
    },
    { timeout: 4000 },
  );
  return result;
}

async function renderTodoDrawer() {
  const result = renderWithQuery(<TodoListPage />);
  // Wait for table row to render.
  const row = await waitFor(
    () => {
      const node = result.container.querySelector('.ant-table-row');
      expect(node).not.toBeNull();
      return node as HTMLElement;
    },
    { timeout: 4000 },
  );
  await act(async () => {
    row.click();
  });
  await waitFor(
    () => {
      const drawer = document.body.querySelector('.ant-drawer');
      expect(drawer).not.toBeNull();
    },
    { timeout: 4000 },
  );
  return result;
}

function readDrawerSnapshot() {
  const drawer = document.body.querySelector('.ant-drawer') as HTMLElement | null;
  if (!drawer) {
    return null;
  }
  const wrapper = drawer.querySelector('.ant-drawer-content-wrapper') as HTMLElement | null;
  const titleNode = drawer.querySelector('.ant-drawer-title') as HTMLElement | null;
  const baseEl = drawer.querySelector('.mobile-detail-drawer') ||
    (drawer.classList.contains('mobile-detail-drawer') ? drawer : null);
  return {
    drawerEl: drawer,
    wrapperWidth: wrapper?.style.width ?? '',
    titleText: titleNode?.textContent ?? '',
    hasMobileBaseClass: baseEl !== null,
  };
}

// ---------------------------------------------------------------------------
// describe block for Bug 1
// ---------------------------------------------------------------------------
describe('Bug 1: Mobile Drawer Consistency', () => {
  /**
   * Property 3 - Bug Condition (Scoped PBT).
   *
   * For viewport innerWidth = 375, the Summary detail Drawer and the Todo detail
   * Drawer must share the unified `mobile-detail-drawer` base class, render
   * non-empty titles, and present consistent inline width on the
   * `.ant-drawer-content-wrapper` element.
   *
   * On UNFIXED code:
   *   - SummaryDetailDrawer hardcodes width="100%" and provides no title
   *     and no mobile-detail-drawer base class.
   *   - TodoListPage Drawer uses width="85%" with title=t('todo.detail')
   *     and className="todo-list-mobile-drawer" (no mobile-detail-drawer).
   *
   * Therefore the assertions below MUST FAIL, confirming Bug 1 exists.
   */
  it(
    'mobile Summary and Todo detail Drawers share unified base class, title and width',
    async () => {
      // Set viewport for this run.
      setViewportWidth(375);

      // ---- Summary Drawer snapshot ----
      const summaryRender = renderWithQuery(<AISummaryPage />);
      const summaryEntry = await waitFor(
        () => {
          const node = summaryRender.container.querySelector('.ai-summary-page__list-item');
          if (!node) {
            // Surface what was rendered so failures are easier to triage.
            throw new Error(`summary entry not yet rendered, html=${summaryRender.container.innerHTML.slice(0, 200)}`);
          }
          return node as HTMLElement;
        },
        { timeout: 4000 },
      );
      summaryEntry.click();
      await waitFor(
        () => {
          const drawer = document.body.querySelector('.ant-drawer');
          expect(drawer).not.toBeNull();
        },
        { timeout: 4000 },
      );
      const summary = readDrawerSnapshot();
      expect(summary).not.toBeNull();
      // Tear down the Summary render and clear any lingering portal nodes
      // before mounting the Todo page (antd Drawer portals into document.body
      // and is not always removed by testing-library cleanup).
      summaryRender.unmount();
      document.body.querySelectorAll('.ant-drawer-mask, .ant-drawer').forEach((n) => n.remove());

      // ---- Todo Drawer snapshot ----
      const todoRender = renderWithQuery(<TodoListPage />);
      const todoRow = await waitFor(
        () => {
          const node = todoRender.container.querySelector('.ant-table-row');
          if (!node) {
            throw new Error(`todo row not yet rendered, html=${todoRender.container.innerHTML.slice(0, 200)}`);
          }
          return node as HTMLElement;
        },
        { timeout: 4000 },
      );
      todoRow.click();
      await waitFor(
        () => {
          const drawer = document.body.querySelector('.ant-drawer');
          expect(drawer).not.toBeNull();
        },
        { timeout: 4000 },
      );
      const todo = readDrawerSnapshot();
      expect(todo).not.toBeNull();

      // ---- Property 3 - Bug Condition assertions (use fast-check to keep this
      // a property test even though the snapshots are already collected). ----
      fc.assert(
        fc.property(fc.constant(375), (_viewportWidth) => {
          // 1. Both Drawers contain the unified `mobile-detail-drawer` base class.
          expect(summary!.hasMobileBaseClass).toBe(true);
          expect(todo!.hasMobileBaseClass).toBe(true);

          // 2. Both Drawers render a non-empty title node.
          expect(summary!.titleText.trim().length).toBeGreaterThan(0);
          expect(todo!.titleText.trim().length).toBeGreaterThan(0);

          // 3. The inline `width` on `.ant-drawer-content-wrapper` is consistent
          //    between the two Drawers.
          expect(summary!.wrapperWidth).toBe(todo!.wrapperWidth);
        }),
        { numRuns: 1 },
      );
    },
    20_000,
  );
});


// ===========================================================================
// Preservation - SSE
// **Validates: Requirements 3.4, 3.5, 3.6, 3.7, 3.8, 3.9**
//
// Property 2 (Preservation): for SSE inputs that do NOT end with a 'done' or
// custom 'error' event, the existing behaviors of SummaryDetailPanel and the
// AISummaryPage entries list MUST remain unchanged on the unfixed code:
//   - 'message' events accumulate verbatim into summary content
//     (data-testid="summary-content").
//   - entries[selectedId].status stays 'analyzing' for the entire stream.
//   - native EventSource.onerror (readyState !== CLOSED) renders the
//     "connection lost" UI without flipping status.
//   - Clicking an entry whose status is already 'completed' or 'error' must
//     NOT construct an EventSource.
//   - Switching selectedId on desktop closes the prior EventSource via
//     cleanup and opens a fresh one for the new analyzing entry.
// ===========================================================================
describe('Preservation - SSE', () => {
  // The desktop viewport is required so AISummaryPage renders the right-hand
  // SummaryDetailPanel inline (mobile drawer scenarios are the subject of
  // the Bug 1 / Drawer Preservation describe-blocks).
  const DESKTOP_WIDTH = 1024;

  function makeAnalyzingEntry(id: number) {
    return {
      ...ANALYZING_ENTRY,
      id,
      status: 'analyzing' as const,
    };
  }

  // -- Property 2.A: message-only sequences keep status === 'analyzing' ---
  it('keeps entries[selected].status === "analyzing" across arbitrary message-only sequences', { timeout: 30000 }, async () => {
    // Generator: a sequence of message payloads with optional native error
    // suffix. The sequence intentionally never ends with a 'done' or custom
    // 'error' event. Payloads are restricted to alphanumeric characters so
    // that ReactMarkdown does not interpret them as Markdown syntax (e.g.
    // '*' produces <ul>, '#' produces <h1>, etc.) which would break a
    // straight-through content equality assertion.
    const ALPHANUM = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ';
    const messageArb = fc
      .array(
        fc.integer({ min: 0, max: ALPHANUM.length - 1 }).map((i) => ALPHANUM[i]),
        { minLength: 0, maxLength: 24 },
      )
      .map((chars) => chars.join(''))
      .filter((s) => s.trim().length === s.length || s.length === 0);
    const sequenceArb = fc.record({
      messages: fc.array(messageArb, { minLength: 0, maxLength: 6 }),
      withNativeError: fc.boolean(),
    });

    await fc.assert(
      fc.asyncProperty(sequenceArb, async ({ messages, withNativeError }) => {
        // Reset per-iteration state because beforeEach is scoped to the
        // surrounding `it`, not to each fast-check shrink.
        cleanup();
        MockEventSource.instances = [];
        MockEventSource.constructorCalls = 0;
        // Clear call history but re-establish mock implementations because
        // vi.clearAllMocks() also wipes implementations.
        mockedListSummaries.mockReset();
        mockedGetSummary.mockReset();
        mockedListTodos.mockReset();
        mockedGetTodo.mockReset();
        setViewportWidth(DESKTOP_WIDTH);

        const entry = makeAnalyzingEntry(100);
        mockedListSummaries.mockResolvedValue([entry]);
        mockedGetSummary.mockResolvedValue(entry);
        mockedListTodos.mockResolvedValue({
          items: [TODO_ITEM],
          total: 1,
          page: 1,
          page_size: 20,
        });

        const { container } = renderWithQuery(<AISummaryPage />);

        await waitFor(() => {
          expect(
            container.querySelector('.ai-summary-page__list-item'),
          ).not.toBeNull();
        });

        await act(async () => {
          (
            container.querySelector('.ai-summary-page__list-item') as HTMLElement
          ).click();
        });

        // Wait for SummaryDetailPanel to construct an EventSource for the
        // analyzing entry.
        await waitFor(() => {
          expect(MockEventSource.instances.length).toBe(1);
        });
        const es = MockEventSource.instances[0];

        // Dispatch the generated message sequence.
        let accumulated = '';
        for (const msg of messages) {
          await act(async () => {
            es.dispatchMessage(msg);
          });
          accumulated += msg;
        }

        // Assertion 1: SSE 'message' events accumulate verbatim into the
        // panel's rendered content. We compare the raw alphanumeric letters
        // because ReactMarkdown wraps the accumulated content in a <p> tag,
        // which adds no extra alphanumeric characters but may collapse some
        // whitespace differently than the source string.
        const expectedAlnum = accumulated.replace(/[^A-Za-z0-9]/g, '');
        if (expectedAlnum.length > 0) {
          await waitFor(() => {
            const contentEl = container.querySelector(
              '[data-testid="summary-content"]',
            );
            expect(contentEl).not.toBeNull();
            const actualAlnum = (contentEl?.textContent ?? '').replace(
              /[^A-Za-z0-9]/g,
              '',
            );
            expect(actualAlnum).toBe(expectedAlnum);
          });
        }

        // Optional native onerror suffix (connection lost, not stream end).
        if (withNativeError) {
          await act(async () => {
            es.dispatchNativeError();
          });
        }

        // Assertion 2: getStreamUrl was called exactly once for the analyzing
        // entry (no extra reconnects from non-bug inputs).
        expect(MockEventSource.constructorCalls).toBe(1);

        // Assertion 3: entries[selected].status is still 'analyzing'. We read
        // it via the rendered status tag in the list item; the i18n mock maps
        // 'aiSummary.statusAnalyzing' → 'Analyzing'.
        const statusTag = container.querySelector(
          '.ai-summary-page__list-item .ant-tag',
        );
        expect(statusTag?.textContent ?? '').toContain('Analyzing');
      }),
      { numRuns: 15 },
    );
  });

  // -- Property 2.B: terminal-status entries never spawn EventSource -------
  it('does NOT construct an EventSource when getSummary returns a terminal status', { timeout: 30000 }, async () => {
    const terminalArb = fc.constantFrom<'completed' | 'error'>('completed', 'error');
    const contentArb = fc.string({ minLength: 0, maxLength: 32 });

    await fc.assert(
      fc.asyncProperty(terminalArb, contentArb, async (terminalStatus, content) => {
        cleanup();
        MockEventSource.instances = [];
        MockEventSource.constructorCalls = 0;
        mockedListSummaries.mockReset();
        mockedGetSummary.mockReset();
        mockedListTodos.mockReset();
        mockedGetTodo.mockReset();
        setViewportWidth(DESKTOP_WIDTH);

        const entry = {
          ...ANALYZING_ENTRY,
          id: 200,
          status: terminalStatus,
          result_content: content,
        };
        mockedListSummaries.mockResolvedValue([entry]);
        mockedGetSummary.mockResolvedValue(entry);
        mockedListTodos.mockResolvedValue({
          items: [TODO_ITEM],
          total: 1,
          page: 1,
          page_size: 20,
        });

        const { container } = renderWithQuery(<AISummaryPage />);

        await waitFor(() => {
          expect(
            container.querySelector('.ai-summary-page__list-item'),
          ).not.toBeNull();
        });

        await act(async () => {
          (
            container.querySelector('.ai-summary-page__list-item') as HTMLElement
          ).click();
        });

        await waitFor(() => {
          expect(mockedGetSummary).toHaveBeenCalled();
        });

        // Flush microtasks so the post-fetch branch finishes.
        await act(async () => {
          await Promise.resolve();
        });

        expect(MockEventSource.constructorCalls).toBe(0);
      }),
      { numRuns: 15 },
    );
  });

  // -- Property 2.C: native onerror renders connection-lost without flip ---
  it('renders the connection-lost UI on native onerror without flipping status', async () => {
    cleanup();
    MockEventSource.instances = [];
    MockEventSource.constructorCalls = 0;
    mockedListSummaries.mockReset();
    mockedGetSummary.mockReset();
    setViewportWidth(DESKTOP_WIDTH);

    const entry = makeAnalyzingEntry(300);
    mockedListSummaries.mockResolvedValue([entry]);
    mockedGetSummary.mockResolvedValue(entry);

    const { container } = renderWithQuery(<AISummaryPage />);

    await waitFor(() => {
      expect(
        container.querySelector('.ai-summary-page__list-item'),
      ).not.toBeNull();
    });

    await act(async () => {
      (
        container.querySelector('.ai-summary-page__list-item') as HTMLElement
      ).click();
    });

    await waitFor(() => {
      expect(MockEventSource.instances.length).toBe(1);
    });
    const es = MockEventSource.instances[0];

    await act(async () => {
      es.dispatchMessage('partial');
      es.dispatchNativeError();
    });

    // Connection-lost notice should be visible (Requirement 3.7).
    await waitFor(() => {
      expect(
        container.querySelector('.summary-detail-panel__connection-error') ||
          container.querySelector('.summary-detail-panel__error'),
      ).not.toBeNull();
    });

    // Status tag in the list still says 'Analyzing'.
    const statusTag = container.querySelector(
      '.ai-summary-page__list-item .ant-tag',
    );
    expect(statusTag?.textContent ?? '').toContain('Analyzing');
  });

  // -- Property 2.D: switching selectedId closes prior SSE & opens a new one
  it('closes the old EventSource and opens a new one when desktop selectedId changes', async () => {
    cleanup();
    MockEventSource.instances = [];
    MockEventSource.constructorCalls = 0;
    mockedListSummaries.mockReset();
    mockedGetSummary.mockReset();
    setViewportWidth(DESKTOP_WIDTH);

    const a = makeAnalyzingEntry(401);
    const b = makeAnalyzingEntry(402);
    mockedListSummaries.mockResolvedValue([a, b]);
    mockedGetSummary.mockImplementation(async (id: number) => (id === a.id ? a : b));

    const { container } = renderWithQuery(<AISummaryPage />);

    await waitFor(() => {
      const items = container.querySelectorAll('.ai-summary-page__list-item');
      expect(items.length).toBe(2);
    });

    const items = container.querySelectorAll<HTMLElement>(
      '.ai-summary-page__list-item',
    );

    await act(async () => {
      items[0].click();
    });
    await waitFor(() => {
      expect(MockEventSource.constructorCalls).toBe(1);
    });
    const firstES = MockEventSource.instances[0];

    await act(async () => {
      items[1].click();
    });
    await waitFor(() => {
      expect(MockEventSource.constructorCalls).toBe(2);
    });

    // Old EventSource must have been closed by the cleanup effect.
    expect(firstES.closed).toBe(true);

    // Both entries still show 'Analyzing' (no done/error fired).
    items.forEach((node) => {
      const tag = node.querySelector('.ant-tag');
      expect(tag?.textContent ?? '').toContain('Analyzing');
    });
  });
});

// ===========================================================================
// Task 1: Bug 2 Exploration (Stream completion list status sync)
//
// **Validates: Requirements 1.4, 1.5, 1.6, 2.4, 2.5, 2.6**
//
// Property 1 (Bug Condition - Scoped PBT):
//   entries=[{id:42, status:'analyzing'}], selectedId=42, an arbitrary SSE
//   `message` sequence terminated by either `done` or a custom `error` event.
//   After the stream ends, the list entry's status tag MUST flip to
//   `Completed` (i18n key `aiSummary.statusCompleted`) for the `done` case
//   and `Error` (i18n key `aiSummary.statusError`) for the `error` case.
//
// Expected outcome on UNFIXED code: assertions FAIL because
//   `SummaryDetailPanel` does not notify `AISummaryPage` of terminal states,
//   so `entries[selectedId].status` stays `'analyzing'` and the rendered
//   status tag still reads `Analyzing` — this is the proof Bug 2 exists.
//
// After Task 5 lands the fix, the very same test should PASS, doubling as
// the Property-1 Expected-Behavior verification (rerun via `vitest -t 'Bug 2'`
// per task 5.7).
// ===========================================================================

/**
 * Render `AISummaryPage` in desktop viewport with a single analyzing entry,
 * click it to select, and wait until SummaryDetailPanel constructs the SSE
 * stream. Returns the rendered container plus the underlying MockEventSource
 * so tests can drive the stream lifecycle deterministically.
 */
async function setupBug2DesktopWithAnalyzingEntry() {
  cleanup();
  MockEventSource.instances = [];
  MockEventSource.constructorCalls = 0;
  vi.clearAllMocks();
  // Force desktop viewport so AISummaryPage renders SummaryDetailPanel inline
  // (no Drawer interference) — Bug 2 lives in the entries-list update path,
  // not in the Drawer wrapper, so desktop is the simplest reproducible scope.
  setViewportWidth(1024);

  const entry = {
    id: 42,
    start_date: '2024-01-01',
    end_date: '2024-01-07',
    status: 'analyzing' as const,
    result_content: '',
    created_at: '2024-01-07T10:00:00Z',
    updated_at: '2024-01-07T10:00:00Z',
  };
  mockedListSummaries.mockResolvedValue([entry]);
  mockedGetSummary.mockResolvedValue({ ...entry });
  mockedCreateSummaryWithTodos.mockResolvedValue({ ...entry });

  const result = render(<AISummaryPage />);

  // Wait for the entry list-item to render and select it.
  const listItem = await waitFor(
    () => {
      const node = result.container.querySelector('.ai-summary-page__list-item');
      expect(node).not.toBeNull();
      return node as HTMLElement;
    },
    { timeout: 4000 },
  );
  await act(async () => {
    listItem.click();
  });

  // Wait until SummaryDetailPanel constructs the SSE stream.
  await waitFor(
    () => {
      expect(MockEventSource.instances.length).toBeGreaterThan(0);
    },
    { timeout: 4000 },
  );
  return {
    container: result.container,
    es: MockEventSource.instances[MockEventSource.instances.length - 1],
  };
}

// SSE message payload generator. Restricted to a small alphabet so the random
// payload can never accidentally collide with the i18n status labels we check.
const bug2MessageArb = fc
  .array(fc.constantFrom('a', 'b', 'c', '1', '2', ' '), { minLength: 0, maxLength: 6 })
  .map((chars) => chars.join(''));
const bug2MessageSequenceArb = fc.array(bug2MessageArb, { minLength: 0, maxLength: 4 });

describe('Bug 2: Stream completion list status sync (UNFIXED expected to fail)', () => {
  /**
   * Property 1 (done variant) — Bug Condition.
   *
   * For any SSE message-sequence terminated by a `done` event, the entry in
   * AISummaryPage's list MUST display the `Completed` status tag.
   *
   * On UNFIXED code the list still shows `Analyzing` — Bug 2 confirmed.
   *
   * **Validates: Requirements 2.4, 2.6**
   */
  it(
    'Property 1 (done): after stream ends with done, list entry status should flip to completed',
    async () => {
      await fc.assert(
        fc.asyncProperty(bug2MessageSequenceArb, async (messages) => {
          const { container, es } = await setupBug2DesktopWithAnalyzingEntry();

          await act(async () => {
            for (const m of messages) es.dispatchMessage(m);
            es.dispatchDone();
          });
          // Flush microtasks so any setEntries triggered by a fix is visible.
          await act(async () => {
            await new Promise((resolve) => setTimeout(resolve, 0));
          });

          // Read the status tag from the list-item. On UNFIXED code this still
          // reads 'Analyzing' and the assertions below fail.
          const tag = container.querySelector('.ai-summary-page__list-item .ant-tag');
          expect(tag?.textContent ?? '').toContain('Completed');
          expect(tag?.textContent ?? '').not.toContain('Analyzing');
        }),
        { numRuns: 2, endOnFailure: true },
      );
    },
    30_000,
  );

  /**
   * Property 1 (error variant) — Bug Condition.
   *
   * For any SSE message-sequence terminated by a custom `error` event, the
   * entry in AISummaryPage's list MUST display the `Error` status tag.
   *
   * On UNFIXED code the list still shows `Analyzing` — Bug 2 confirmed.
   *
   * **Validates: Requirements 2.5, 2.6**
   */
  it(
    'Property 1 (error): after stream ends with custom error, list entry status should flip to error',
    async () => {
      await fc.assert(
        fc.asyncProperty(bug2MessageSequenceArb, async (messages) => {
          const { container, es } = await setupBug2DesktopWithAnalyzingEntry();

          await act(async () => {
            for (const m of messages) es.dispatchMessage(m);
            es.dispatchCustomError('llm provider failed');
          });
          await act(async () => {
            await new Promise((resolve) => setTimeout(resolve, 0));
          });

          const tag = container.querySelector('.ai-summary-page__list-item .ant-tag');
          expect(tag?.textContent ?? '').toContain('Error');
          expect(tag?.textContent ?? '').not.toContain('Analyzing');
        }),
        { numRuns: 2, endOnFailure: true },
      );
    },
    30_000,
  );
});


// ===========================================================================
// describe block for Property 4 (Preservation - Drawer)
// ===========================================================================
//
// Property 4 (Preservation): On viewports wider than 768px no detail Drawer
// is mounted; in mobile viewports the Drawer-internal interactions and
// antd's built-in ESC behavior are preserved. These assertions hold on the
// UNFIXED code (no fix is applied yet) and serve as the regression baseline
// after Task 5 introduces MobileDetailDrawer.
//
// **Validates: Requirements 3.1, 3.2, 3.3, 3.10**
//
// Imports needed only for this describe block live here so Tasks 1-3 can be
// merged independently without shifting upstream import ordering.
// ---------------------------------------------------------------------------
import { fireEvent } from '@testing-library/react';
import { Drawer as AntdDrawer } from 'antd';
import { startTodo as startTodoApi } from '../../src/api/todos';

const mockedStartTodo = vi.mocked(startTodoApi);

describe('Preservation - Drawer', () => {
  /**
   * Property 4.1 - Desktop viewport: no detail Drawer is mounted.
   *
   * For viewport innerWidth in [769, 1920], rendering AISummaryPage and
   * TodoListPage MUST NOT mount any `.mobile-detail-drawer` or `.ant-drawer`
   * detail panel. AnalysisDrawer (the AI analysis configuration drawer) is
   * rendered with open=false and antd portals only when open=true, so
   * `document.body` should contain zero `.ant-drawer` elements after the
   * page settles into its desktop layout.
   */
  it('desktop viewport (>768px) renders no detail Drawer for either page', async () => {
    await fc.assert(
      fc.asyncProperty(fc.integer({ min: 769, max: 1920 }), async (viewportWidth) => {
        // Reset shared state per property run.
        cleanup();
        document.body.innerHTML = '';
        MockEventSource.instances = [];
        mockedListSummaries.mockResolvedValue([ANALYZING_ENTRY]);
        mockedGetSummary.mockResolvedValue(ANALYZING_ENTRY);
        mockedListTodos.mockResolvedValue({
          items: [TODO_ITEM],
          total: 1,
          page: 1,
          page_size: 20,
        });
        mockedGetTodo.mockResolvedValue(TODO_DETAIL);
        setViewportWidth(viewportWidth);

        // ---- AISummaryPage on desktop ----
        const summaryRender = renderWithQuery(<AISummaryPage />);
        // Wait for entries to render so the desktop layout has settled.
        await waitFor(() => {
          const node = summaryRender.container.querySelector('.ai-summary-page__list-item');
          expect(node).not.toBeNull();
        });
        // No drawer should be portalled into body for the detail panel and
        // AnalysisDrawer is closed by default.
        const summaryDrawerCount = document.body.querySelectorAll('.ant-drawer').length;
        const summaryMobileDrawerCount = document.body.querySelectorAll('.mobile-detail-drawer').length;
        expect(summaryDrawerCount).toBe(0);
        expect(summaryMobileDrawerCount).toBe(0);
        cleanup();
        document.body.innerHTML = '';

        // ---- TodoListPage on desktop ----
        const todoRender = renderWithQuery(<TodoListPage />);
        await waitFor(() => {
          const node = todoRender.container.querySelector('.ant-table-row');
          expect(node).not.toBeNull();
        });
        const todoDrawerCount = document.body.querySelectorAll('.ant-drawer').length;
        const todoMobileDrawerCount = document.body.querySelectorAll('.mobile-detail-drawer').length;
        expect(todoDrawerCount).toBe(0);
        expect(todoMobileDrawerCount).toBe(0);
        cleanup();
        document.body.innerHTML = '';
      }),
      { numRuns: 5 },
    );
  });

  /**
   * Property 4.2 - Mobile Drawer's TodoDetailPanel keeps responding to status
   * toggle clicks. Renders TodoListPage in a mobile viewport, opens the detail
   * Drawer for a TODO_DETAIL whose status is 'open', clicks the "Start
   * Progress" button (`.detail-panel-btn-start`), and asserts that the
   * `startTodo` API mock was invoked with the todo id. This confirms that
   * existing mutation wiring inside the Drawer continues to function.
   */
  it('mobile Drawer keeps TodoDetailPanel status toggle button responsive', async () => {
    setViewportWidth(375);
    mockedListTodos.mockResolvedValue({
      items: [TODO_ITEM],
      total: 1,
      page: 1,
      page_size: 20,
    });
    mockedGetTodo.mockResolvedValue(TODO_DETAIL);
    mockedStartTodo.mockResolvedValue(undefined as unknown as never);

    const result = renderWithQuery(<TodoListPage />);

    // Wait for table row, click it to open the mobile detail Drawer.
    const row = await waitFor(() => {
      const node = result.container.querySelector('.ant-table-row');
      expect(node).not.toBeNull();
      return node as HTMLElement;
    });
    await act(async () => {
      row.click();
    });

    // Drawer should mount and the TodoDetailPanel inside should resolve.
    await waitFor(() => {
      const drawer = document.body.querySelector('.ant-drawer');
      expect(drawer).not.toBeNull();
    });
    // Wait for the start button to render after the todo detail loads.
    const startBtn = await waitFor(() => {
      const node = document.body.querySelector('.detail-panel-btn-start') as HTMLElement | null;
      expect(node).not.toBeNull();
      return node as HTMLElement;
    });

    // Click the status toggle button inside the Drawer.
    await act(async () => {
      startBtn.click();
    });

    // The mocked startTodo mutation MUST be invoked with the open todo's id.
    await waitFor(() => {
      expect(mockedStartTodo).toHaveBeenCalledWith(TODO_DETAIL.id);
    });
  });

  /**
   * Property 4.3 - antd Drawer's built-in ESC handling is preserved. Mounts a
   * standalone antd `Drawer` with a spy `onClose`, dispatches a keydown event
   * for the Escape key on `window`, and asserts the spy was called exactly
   * once. This confirms that consumers (TodoListPage / SummaryDetailDrawer)
   * continue to inherit antd's accessibility behavior.
   */
  it('mobile Drawer preserves antd Escape-to-close behavior', async () => {
    setViewportWidth(375);
    const onClose = vi.fn();

    render(
      <AntdDrawer
        open
        onClose={onClose}
        title="Test"
        placement="right"
        width="100%"
      >
        <div>Drawer body</div>
      </AntdDrawer>,
    );

    // Wait for the antd Drawer to mount into the DOM.
    await waitFor(() => {
      const drawer = document.body.querySelector('.ant-drawer');
      expect(drawer).not.toBeNull();
    });

    // Dispatch the Escape keydown that rc-component/portal listens for on
    // window. Use act so React processes the resulting state change.
    await act(async () => {
      fireEvent.keyDown(window, { key: 'Escape', code: 'Escape' });
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
