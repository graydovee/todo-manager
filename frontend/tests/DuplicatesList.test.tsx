import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { DuplicatesList } from '../src/components/DuplicatesList'
import type { TodoSummary } from '../src/types'

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'detail.duplicateOf': 'Duplicate of',
        'detail.duplicates': 'Duplicates',
        'todo.open': 'Open',
        'todo.inProgress': 'In Progress',
        'todo.completed': 'Completed',
        'todo.duplicate': 'Duplicate',
      }
      return translations[key] || key
    },
  }),
}))

/**
 * Validates: Requirements 4.3
 * Property 7: Duplicate list items display all required fields
 */
describe('DuplicatesList', () => {
  const mockNavigate = vi.fn()

  const makeSummary = (overrides: Partial<TodoSummary> = {}): TodoSummary => ({
    id: 1,
    code: '001',
    title: 'Test Todo',
    category: 'bug',
    status: 'open',
    ...overrides,
  })

  describe('hidden when no duplicates exist', () => {
    it('renders nothing when duplicateOf is null and duplicates is empty', () => {
      const { container } = render(
        <DuplicatesList duplicateOf={null} duplicates={[]} onNavigate={mockNavigate} />
      )
      expect(container.innerHTML).toBe('')
    })
  })

  describe('duplicateOf section', () => {
    it('renders "Duplicate of" section with status tag, code, and title', () => {
      const canonical: TodoSummary = makeSummary({
        id: 10,
        code: '042',
        title: 'Canonical Task',
        category: 'feature',
        status: 'in_progress',
      })

      const { container } = render(
        <DuplicatesList duplicateOf={canonical} duplicates={[]} onNavigate={mockNavigate} />
      )

      // Should show "Duplicate of" label
      expect(container.textContent).toContain('Duplicate of')

      // Should show status tag with translated text
      expect(container.textContent).toContain('In Progress')

      // Should show formatted code (F-042 for feature category)
      expect(container.textContent).toContain('F-042')

      // Should show title
      expect(container.textContent).toContain('Canonical Task')
    })

    it('clicking duplicateOf item calls onNavigate with correct id', () => {
      const canonical: TodoSummary = makeSummary({ id: 99 })

      const { container } = render(
        <DuplicatesList duplicateOf={canonical} duplicates={[]} onNavigate={mockNavigate} />
      )

      const link = container.querySelector('a')
      expect(link).not.toBeNull()
      fireEvent.click(link!)
      expect(mockNavigate).toHaveBeenCalledWith(99)
    })
  })

  describe('duplicates list section', () => {
    it('renders each duplicate item with status tag, formatted code, and title', () => {
      const duplicates: TodoSummary[] = [
        makeSummary({ id: 2, code: '005', title: 'Dup One', category: 'task', status: 'duplicate' }),
        makeSummary({ id: 3, code: '012', title: 'Dup Two', category: 'bug', status: 'open' }),
        makeSummary({ id: 4, code: '100', title: 'Dup Three', category: 'feature', status: 'completed' }),
      ]

      const { container } = render(
        <DuplicatesList duplicateOf={null} duplicates={duplicates} onNavigate={mockNavigate} />
      )

      // Should show "Duplicates" label
      expect(container.textContent).toContain('Duplicates')

      // First item: task category -> T-005, status duplicate
      expect(container.textContent).toContain('T-005')
      expect(container.textContent).toContain('Dup One')
      expect(container.textContent).toContain('Duplicate')

      // Second item: bug category -> B-012, status open
      expect(container.textContent).toContain('B-012')
      expect(container.textContent).toContain('Dup Two')
      expect(container.textContent).toContain('Open')

      // Third item: feature category -> F-100, status completed
      expect(container.textContent).toContain('F-100')
      expect(container.textContent).toContain('Dup Three')
      expect(container.textContent).toContain('Completed')
    })

    it('clicking a duplicate item calls onNavigate with the correct id', () => {
      const duplicates: TodoSummary[] = [
        makeSummary({ id: 7, code: '003', title: 'Click Me', category: 'bug', status: 'open' }),
      ]

      const { container } = render(
        <DuplicatesList duplicateOf={null} duplicates={duplicates} onNavigate={mockNavigate} />
      )

      const link = container.querySelector('a')
      expect(link).not.toBeNull()
      fireEvent.click(link!)
      expect(mockNavigate).toHaveBeenCalledWith(7)
    })
  })
})
