import { useState } from 'react';
import { Button, Input, Tooltip } from 'antd';
import {
  EditOutlined,
  ReloadOutlined,
  CheckOutlined,
  CloseOutlined,
  LoadingOutlined,
  LeftOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ReactMarkdown from 'react-markdown';
import './FollowupMessageBubble.css';

export interface FollowupMessageBubbleProps {
  question: string;
  answer: string;
  isStreaming?: boolean;
  isError?: boolean;
  onEdit?: (newQuestion: string) => void;
  onRegenerate?: () => void;
  onRetry?: () => void;
  // Version navigation props (for task 9.5)
  versions?: { content: string; version_number: number }[];
  currentVersionIndex?: number;
  onVersionChange?: (index: number) => void;
}

const MAX_EDIT_LENGTH = 2000;

export function FollowupMessageBubble({
  question,
  answer,
  isStreaming,
  isError,
  onEdit,
  onRegenerate,
  onRetry,
  versions,
  currentVersionIndex,
  onVersionChange,
}: FollowupMessageBubbleProps) {
  const { t } = useTranslation();
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(question);

  // Enter edit mode
  const handleEditClick = () => {
    setEditValue(question);
    setIsEditing(true);
  };

  // Confirm edit
  const handleEditConfirm = () => {
    const trimmed = editValue.trim();
    if (trimmed && onEdit) {
      onEdit(trimmed);
    }
    setIsEditing(false);
  };

  // Cancel edit
  const handleEditCancel = () => {
    setEditValue(question);
    setIsEditing(false);
  };

  // Handle Enter key in edit input
  const handleEditKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleEditConfirm();
    } else if (e.key === 'Escape') {
      handleEditCancel();
    }
  };

  const isEditConfirmDisabled = !editValue.trim();

  // Determine if version navigation should be shown
  const showVersionNav = versions && versions.length > 1;
  const isFirstVersion = currentVersionIndex === 0;
  const isLastVersion = versions ? currentVersionIndex === versions.length - 1 : true;

  // Determine the displayed answer content: use versioned content when available
  const displayedAnswer = versions && versions.length > 0 && currentVersionIndex !== undefined
    ? versions[currentVersionIndex].content
    : answer;

  return (
    <div className="followup-bubble" data-testid="followup-message-bubble">
      {/* User question bubble - right side */}
      <div className="followup-bubble__user-row">
        {isEditing ? (
          <div className="followup-bubble__edit-container">
            <Input.TextArea
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onKeyDown={handleEditKeyDown}
              maxLength={MAX_EDIT_LENGTH}
              autoSize={{ minRows: 1, maxRows: 6 }}
              autoFocus
              data-testid="followup-edit-input"
            />
            <div className="followup-bubble__edit-actions">
              <Button
                type="primary"
                size="small"
                icon={<CheckOutlined />}
                onClick={handleEditConfirm}
                disabled={isEditConfirmDisabled}
                data-testid="followup-edit-confirm"
              />
              <Button
                size="small"
                icon={<CloseOutlined />}
                onClick={handleEditCancel}
                data-testid="followup-edit-cancel"
              />
            </div>
          </div>
        ) : (
          <div className="followup-bubble__user-bubble">
            <span className="followup-bubble__question-text">{question}</span>
            <div className="followup-bubble__actions">
              <Tooltip title={t('aiSummary.followup.editTooltip')}>
                <button
                  className="followup-bubble__action-btn"
                  onClick={handleEditClick}
                  disabled={isStreaming}
                  aria-label={t('aiSummary.followup.editTooltip')}
                  data-testid="followup-edit-btn"
                >
                  <EditOutlined />
                </button>
              </Tooltip>
              <Tooltip title={t('aiSummary.followup.regenerateTooltip')}>
                <button
                  className="followup-bubble__action-btn"
                  onClick={onRegenerate}
                  disabled={isStreaming}
                  aria-label={t('aiSummary.followup.regenerateTooltip')}
                  data-testid="followup-regenerate-btn"
                >
                  <ReloadOutlined />
                </button>
              </Tooltip>
            </div>
          </div>
        )}
      </div>

      {/* AI answer bubble - left side */}
      <div className="followup-bubble__ai-row">
        {isStreaming && !displayedAnswer ? (
          <div className="followup-bubble__loading">
            <LoadingOutlined />
            <span>{t('aiSummary.followup.loading')}</span>
          </div>
        ) : isError ? (
          <div className="followup-bubble__error">
            <span>{displayedAnswer}</span>
            {onRetry && (
              <Button
                type="link"
                size="small"
                onClick={onRetry}
                className="followup-bubble__retry-btn"
                data-testid="followup-retry-btn"
              >
                {t('aiSummary.followup.retryButton')}
              </Button>
            )}
          </div>
        ) : displayedAnswer ? (
          <div className="followup-bubble__ai-bubble">
            <ReactMarkdown>{displayedAnswer}</ReactMarkdown>
            {/* Version navigation */}
            {showVersionNav && (
              <div className="followup-bubble__version-nav" data-testid="followup-version-nav">
                <button
                  className="followup-bubble__version-arrow"
                  onClick={() => onVersionChange?.(currentVersionIndex! - 1)}
                  disabled={isFirstVersion}
                  aria-label="Previous version"
                  data-testid="followup-version-prev"
                >
                  <LeftOutlined />
                </button>
                <span className="followup-bubble__version-indicator" data-testid="followup-version-indicator">
                  {t('aiSummary.followup.versionIndicator', {
                    current: (currentVersionIndex ?? 0) + 1,
                    total: versions!.length,
                  })}
                </span>
                <button
                  className="followup-bubble__version-arrow"
                  onClick={() => onVersionChange?.(currentVersionIndex! + 1)}
                  disabled={isLastVersion}
                  aria-label="Next version"
                  data-testid="followup-version-next"
                >
                  <RightOutlined />
                </button>
              </div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}
