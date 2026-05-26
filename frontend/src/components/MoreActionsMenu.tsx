import { Button, Dropdown } from 'antd';
import type { MenuProps } from 'antd';
import {
  DownOutlined,
  PushpinOutlined,
  PushpinFilled,
  HighlightOutlined,
  HighlightFilled,
  PlusOutlined,
  SwapOutlined,
  CopyOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { Status, TodoDetail } from '../types';

export interface MoreActionsMenuProps {
  todo: TodoDetail;
  onJumpToStage: (status: Status) => void;
  onAddPrerequisite: () => void;
  onTogglePin: () => void;
  onToggleHighlight: () => void;
  onMarkDuplicate: () => void;
}

export function MoreActionsMenu({
  todo,
  onJumpToStage,
  onAddPrerequisite,
  onTogglePin,
  onToggleHighlight,
  onMarkDuplicate,
}: MoreActionsMenuProps) {
  const { t } = useTranslation();

  const stageMenuItems = [
    { key: 'open', label: t('todo.open') },
    { key: 'in_progress', label: t('todo.inProgress') },
    { key: 'completed', label: t('todo.completed') },
  ].filter((item) => item.key !== todo.status);

  const menuItems: MenuProps['items'] = [
    {
      key: 'jump-to-stage',
      label: t('detail.jumpToStage'),
      icon: <SwapOutlined />,
      children: stageMenuItems.map((item) => ({
        key: `stage-${item.key}`,
        label: item.label,
      })),
    },
    {
      key: 'add-prerequisite',
      label: t('detail.addPrerequisite'),
      icon: <PlusOutlined />,
    },
    {
      key: 'toggle-pin',
      label: todo.pinned ? t('detail.unpin') : t('detail.pin'),
      icon: todo.pinned ? <PushpinFilled /> : <PushpinOutlined />,
    },
    {
      key: 'toggle-highlight',
      label: todo.highlighted ? t('detail.unhighlight') : t('detail.highlight'),
      icon: todo.highlighted ? <HighlightFilled /> : <HighlightOutlined />,
    },
    {
      key: 'mark-duplicate',
      label: todo.status === 'duplicate' ? t('detail.cannotMarkDuplicate') : t('detail.markDuplicate'),
      icon: <CopyOutlined />,
      disabled: todo.status === 'duplicate',
    },
  ];

  const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'add-prerequisite') {
      onAddPrerequisite();
    } else if (key === 'toggle-pin') {
      onTogglePin();
    } else if (key === 'toggle-highlight') {
      onToggleHighlight();
    } else if (key === 'mark-duplicate') {
      onMarkDuplicate();
    } else if (key.startsWith('stage-')) {
      const status = key.replace('stage-', '') as Status;
      onJumpToStage(status);
    }
  };

  return (
    <Dropdown
      menu={{ items: menuItems, onClick: handleMenuClick }}
      trigger={['click']}
    >
      <Button className="more-actions-menu-trigger">
        {t('detail.moreActions')} <DownOutlined />
      </Button>
    </Dropdown>
  );
}
