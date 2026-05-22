import { Drawer } from 'antd';
import { SummaryDetailPanel } from './SummaryDetailPanel';

interface Props {
  open: boolean;
  onClose: () => void;
  summaryId: number | null;
}

export function SummaryDetailDrawer({ open, onClose, summaryId }: Props) {
  return (
    <Drawer
      placement="right"
      width="100%"
      open={open}
      onClose={onClose}
      destroyOnClose
    >
      <SummaryDetailPanel summaryId={open ? summaryId : null} />
    </Drawer>
  );
}
