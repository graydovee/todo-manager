import { useTranslation } from 'react-i18next';
import { MobileDetailDrawer } from './MobileDetailDrawer';
import { SummaryDetailPanel } from './SummaryDetailPanel';

interface Props {
  open: boolean;
  onClose: () => void;
  summaryId: number | null;
  /**
   * Optional callback forwarded to SummaryDetailPanel so the parent page can
   * react to streaming completion or error events (e.g. refresh history list).
   */
  onStatusChange?: (status: 'completed' | 'error') => void;
}

export function SummaryDetailDrawer({
  open,
  onClose,
  summaryId,
  onStatusChange,
}: Props) {
  const { t } = useTranslation();

  return (
    <MobileDetailDrawer
      open={open}
      onClose={onClose}
      title={t('summaryDetail.drawerTitle')}
    >
      {/*
        Keep the existing "clear summaryId when closed" pattern: when `open`
        is false, pass `null` so the panel resets internal state. Combined
        with MobileDetailDrawer's `destroyOnClose=true` default, this also
        ensures the panel unmounts and any active EventSource is closed.
      */}
      <SummaryDetailPanel
        summaryId={open ? summaryId : null}
        onStatusChange={onStatusChange}
      />
    </MobileDetailDrawer>
  );
}
