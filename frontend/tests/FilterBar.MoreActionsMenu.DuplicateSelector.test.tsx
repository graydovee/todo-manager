import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest'
import { render, cleanup, fireEvent, waitFor } from '@testing-library/react'

// Polyfill ResizeObserver for jsdom (required by Ant Design Dropdown)
beforeAll(() => {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
})

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'filter.searchTitle': 'Search title',
        'filter.codeExact': 'Code exact',
        'filter.tagPlaceholder': 'Tags',
        'filter.tagLoadError': 'Tag load error',
        'filter.tagRetry': 'Retry',
        'filter.updatedAt': 'Updated at',
        'filter.last1Day': 'Last 1 day',
        'filter.last3Days': 'Last 3 days',
        'filter.last7Days': 'Last 7 days',
        'filter.last30Days': 'Last 30 days',
        'filter.sortBy': 'Sort by',
        'filter.created': 'Created',
        'filter.updated': 'Updated',
        'filter.dueDate': 'Due date',
        'filter.desc': 'Desc',
        'filter.asc': 'Asc',
        'todo.category': 'Category',
        'todo.bug': 'Bug',
        'todo.feature': 'Feature',
        'todo.task': 'Task',
        'todo.priority': 'Priority',
        'todo.status': 'Status',
        'todo.open': 'Open',
        'todo.inProgress': 'In Progress',
        'todo.completed': 'Completed',
        'todo.duplicate': 'Duplicate',
        'todo.done': 'Done',
        'todo.enterTodoIds': 'Search todos',
        'common.clear': 'Clear',
        'common.cancel': 'Cancel',
        'detail.moreActions': 'More Actions',
        'detail.jumpToStage': 'Jump to Stage',
        'detail.addPrerequisite': 'Add Prerequisite',
        'detail.pin': 'Pin',
        'detail.unpin': 'Unpin',
        'detail.highlight': 'Highlight',
        'detail.unhighlight': 'Unhighlight',
        'detail.markDuplicate': 'Mark as Duplicate',
        'detail.cannotMarkDuplicate': 'Already Duplicate',
        'confirm.duplicateMarked': 'Marked as duplicate',
        'confirm.duplicateError': 'Duplicate error',
      }
      return translations[key] || key
    },
  }),
}))

// Mock useTags hook
vi.mock('../src/hooks/useTodos', () => ({
  useTags: () => ({
    data: ['tag1', 'tag2'],
    isError: false,
    refetch: vi.fn(),
  }),
  useUpdateTodo: () => ({
    mutateAsync: vi.fn().mockResolvedValue({}),
    isPending: false,
  }),
}))

// Mock the todos API
vi.mock('../src/api/todos', () => ({
  listTodos: vi.fn().mockResolvedValue({
    items: [
      { id: 2, code: '001', title: 'Other Todo', category: 'bug', status: 'open' },
      { id: 3, code: '002', title: 'Another Todo', category: 'feature', status: 'in_progress' },
      { id: 5, code: '003', title: 'Duplicate Todo', category: 'task', status: 'duplicate' },
    ],
    total: 3,
    page: 1,
    page_size: 20,
  }),
}))

import { TodoFilter } from '../src/components/TodoFilter'
import { MoreActionsMenu } from '../src/components/MoreActionsMenu'
import { DuplicateSelector } from '../src/components/DuplicateSelector'
import type { TodoDetail } from '../src/types'

afterEach(() => {
  cleanup()
})

describe('FilterBar', () => {
  it('renders three rows with filter-bar__row class', () => {
    const { container } = render(
      <TodoFilter filters={{}} onChange={vi.fn()} />
    )
    const rows = container.querySelectorAll('.filter-bar__row')
    expect(rows.length).toBe(3)
  })

  it('renders search inputs (title search + code exact) in row 1', () => {
    const { container } = render(
      <TodoFilter filters={{}} onChange={vi.fn()} />
    )
    const rows = container.querySelectorAll('.filter-bar__row')
    const row1 = rows[0]
    // Row 1 should have 2 filter-control elements (title search + code exact)
    const controls = row1.querySelectorAll('.filter-control')
    expect(controls.length).toBe(2)
    // Should contain input elements
    const inputs = row1.querySelectorAll('input')
    expect(inputs.length).toBeGreaterThanOrEqual(2)
  })

  it('renders filter controls in row 2', () => {
    const { container } = render(
      <TodoFilter filters={{}} onChange={vi.fn()} />
    )
    const rows = container.querySelectorAll('.filter-bar__row')
    const row2 = rows[1]
    // Row 2 should have filter controls: tag, category, priority, status, updated_at
    const controls = row2.querySelectorAll('.filter-control')
    expect(controls.length).toBe(5)
  })

  it('renders sort controls in row 3', () => {
    const { container } = render(
      <TodoFilter filters={{}} onChange={vi.fn()} />
    )
    const rows = container.querySelectorAll('.filter-bar__row')
    const row3 = rows[2]
    // Row 3 should have sort_by, sort_order controls + clear button at front
    const controls = row3.querySelectorAll('.filter-control')
    expect(controls.length).toBe(2)
    // Clear button should be present (icon only, no text)
    const clearBtn = row3.querySelector('.filter-clear-btn')
    expect(clearBtn).not.toBeNull()
  })

  it('Updated_At_Filter renders a DatePicker with presets', () => {
    const { container } = render(
      <TodoFilter filters={{}} onChange={vi.fn()} />
    )
    const rows = container.querySelectorAll('.filter-bar__row')
    const row2 = rows[1]
    // The updated_at filter is the 5th filter-control in row 2
    const filterControls = row2.querySelectorAll('.filter-control')
    // The 5th control (index 4) is the updated_at filter
    const updatedAtControl = filterControls[4]
    expect(updatedAtControl).toBeDefined()
    // Ant Design DatePicker renders with .ant-picker class
    const picker = updatedAtControl.querySelector('.ant-picker')
    expect(picker).not.toBeNull()
  })
})

describe('MoreActionsMenu', () => {
  const baseTodo: TodoDetail = {
    id: 1,
    code: '001',
    title: 'Test Todo',
    description: 'Test description',
    category: 'bug',
    priority: 'p1',
    status: 'open',
    due_at: null,
    tags: [],
    pinned: false,
    highlighted: false,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    depends_on: [],
    depended_by: [],
    duplicate_of: null,
    duplicates: [],
  }

  it('renders the More Actions trigger button', () => {
    const { container } = render(
      <MoreActionsMenu
        todo={baseTodo}
        onJumpToStage={vi.fn()}
        onAddPrerequisite={vi.fn()}
        onTogglePin={vi.fn()}
        onToggleHighlight={vi.fn()}
        onMarkDuplicate={vi.fn()}
      />
    )
    const trigger = container.querySelector('.more-actions-menu-trigger')
    expect(trigger).not.toBeNull()
    expect(trigger!.textContent).toContain('More Actions')
  })

  it('shows "Pin" label when todo is not pinned', async () => {
    const { container, getByText } = render(
      <MoreActionsMenu
        todo={{ ...baseTodo, pinned: false }}
        onJumpToStage={vi.fn()}
        onAddPrerequisite={vi.fn()}
        onTogglePin={vi.fn()}
        onToggleHighlight={vi.fn()}
        onMarkDuplicate={vi.fn()}
      />
    )
    // Click the trigger to open the dropdown
    const trigger = container.querySelector('.more-actions-menu-trigger')!
    fireEvent.click(trigger)

    await waitFor(() => {
      expect(getByText('Pin')).toBeDefined()
    })
  })

  it('shows "Unpin" label when todo is pinned', async () => {
    const { container, getByText } = render(
      <MoreActionsMenu
        todo={{ ...baseTodo, pinned: true }}
        onJumpToStage={vi.fn()}
        onAddPrerequisite={vi.fn()}
        onTogglePin={vi.fn()}
        onToggleHighlight={vi.fn()}
        onMarkDuplicate={vi.fn()}
      />
    )
    const trigger = container.querySelector('.more-actions-menu-trigger')!
    fireEvent.click(trigger)

    await waitFor(() => {
      expect(getByText('Unpin')).toBeDefined()
    })
  })

  it('shows "Highlight" label when todo is not highlighted', async () => {
    const { container, getByText } = render(
      <MoreActionsMenu
        todo={{ ...baseTodo, highlighted: false }}
        onJumpToStage={vi.fn()}
        onAddPrerequisite={vi.fn()}
        onTogglePin={vi.fn()}
        onToggleHighlight={vi.fn()}
        onMarkDuplicate={vi.fn()}
      />
    )
    const trigger = container.querySelector('.more-actions-menu-trigger')!
    fireEvent.click(trigger)

    await waitFor(() => {
      expect(getByText('Highlight')).toBeDefined()
    })
  })

  it('shows "Unhighlight" label when todo is highlighted', async () => {
    const { container, getByText } = render(
      <MoreActionsMenu
        todo={{ ...baseTodo, highlighted: true }}
        onJumpToStage={vi.fn()}
        onAddPrerequisite={vi.fn()}
        onTogglePin={vi.fn()}
        onToggleHighlight={vi.fn()}
        onMarkDuplicate={vi.fn()}
      />
    )
    const trigger = container.querySelector('.more-actions-menu-trigger')!
    fireEvent.click(trigger)

    await waitFor(() => {
      expect(getByText('Unhighlight')).toBeDefined()
    })
  })
})

describe('DuplicateSelector', () => {
  it('renders a search select and confirm/cancel buttons', () => {
    const { container, getByText } = render(
      <DuplicateSelector
        currentTodoId={1}
        onSelect={vi.fn()}
        onCancel={vi.fn()}
      />
    )
    // Should have a Select component
    const select = container.querySelector('.ant-select')
    expect(select).not.toBeNull()

    // Should have confirm (Done) and cancel buttons
    expect(getByText('Done')).toBeDefined()
    expect(getByText('Cancel')).toBeDefined()
  })

  it('confirm button is disabled when no selection is made', () => {
    const { container } = render(
      <DuplicateSelector
        currentTodoId={1}
        onSelect={vi.fn()}
        onCancel={vi.fn()}
      />
    )
    // The Done button should be disabled
    const buttons = container.querySelectorAll('button')
    const doneButton = Array.from(buttons).find(btn => btn.textContent === 'Done')
    expect(doneButton).toBeDefined()
    expect(doneButton!.disabled).toBe(true)
  })

  it('calls onCancel when cancel button is clicked', () => {
    const onCancel = vi.fn()
    const { getByText } = render(
      <DuplicateSelector
        currentTodoId={1}
        onSelect={vi.fn()}
        onCancel={onCancel}
      />
    )
    fireEvent.click(getByText('Cancel'))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
