import type { ReactNode } from 'react';
import { Drawer } from 'antd';
import './MobileDetailDrawer.css';

/**
 * Props for the unified mobile detail drawer.
 *
 * This component is a thin wrapper around antd `Drawer` that enforces a
 * consistent set of defaults (placement, width, destroyOnClose, base class)
 * for all mobile detail Drawers in the application. Call sites may override
 * `width`, `destroyOnClose`, or append extra classes via `className`.
 */
export interface MobileDetailDrawerProps {
  /** Whether the drawer is open. */
  open: boolean;
  /** Close handler invoked by antd Drawer (close button, ESC, mask click). */
  onClose: () => void;
  /** Optional drawer header title. Both call sites are expected to pass one. */
  title?: ReactNode;
  /** Drawer body content. */
  children: ReactNode;
  /** Override default width. Defaults to '100%' for mobile fullscreen. */
  width?: string | number;
  /** Override default destroyOnClose. Defaults to true. */
  destroyOnClose?: boolean;
  /** Optional extra className appended after the unified base class. */
  className?: string;
}

const BASE_CLASS_NAME = 'mobile-detail-drawer';

/**
 * Unified mobile detail drawer component shared by AISummaryPage and
 * TodoListPage. Encapsulates the antd Drawer configuration so both call sites
 * render with identical width, header, close button, and padding behavior.
 */
export function MobileDetailDrawer({
  open,
  onClose,
  title,
  children,
  width,
  destroyOnClose,
  className,
}: MobileDetailDrawerProps) {
  // Merge the base class with any caller-provided className via space join.
  const mergedClassName = className
    ? `${BASE_CLASS_NAME} ${className}`
    : BASE_CLASS_NAME;

  return (
    <Drawer
      placement="right"
      width={width ?? '100%'}
      destroyOnClose={destroyOnClose ?? true}
      open={open}
      onClose={onClose}
      title={title}
      className={mergedClassName}
    >
      {children}
    </Drawer>
  );
}
