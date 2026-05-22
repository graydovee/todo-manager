import { useState, useEffect, useCallback } from 'react';
import { Button, Empty, Modal, Spin, Tag, message } from 'antd';
import { DeleteOutlined, LoadingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { createSummaryWithTodos, deleteSummary, listSummaries } from '../api/summaries';
import type { SummaryEntry } from '../api/summaries';
import { AnalysisDrawer } from '../components/AnalysisDrawer';
import { SummaryDetailPanel } from '../components/SummaryDetailPanel';
import { SummaryDetailDrawer } from '../components/SummaryDetailDrawer';
import dayjs from 'dayjs';
import './AISummaryPage.css';

function useIsMobile() {
  const [isMobile, setIsMobile] = useState(() => window.innerWidth <= 768);
  useEffect(() => {
    const handler = () => setIsMobile(window.innerWidth <= 768);
    window.addEventListener('resize', handler);
    return () => window.removeEventListener('resize', handler);
  }, []);
  return isMobile;
}

export function AISummaryPage() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const [entries, setEntries] = useState<SummaryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detailDrawerOpen, setDetailDrawerOpen] = useState(false);

  // Fetch history list on mount
  const fetchEntries = useCallback(async () => {
    try {
      const data = await listSummaries();
      setEntries(data);
    } catch {
      message.error(t('aiSummary.emptyState'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void fetchEntries();
  }, [fetchEntries]);

  const handleStartAnalysis = async (startDate: string, endDate: string, todoIds: number[], language: string) => {
    try {
      const entry = await createSummaryWithTodos(startDate, endDate, todoIds, language);
      setEntries((prev) => [entry, ...prev]);
      setSelectedId(entry.id);
    } catch {
      message.error(t('aiSummary.statusError'));
    }
  };

  const handleEntryClick = (entry: SummaryEntry) => {
    setSelectedId(entry.id);
    if (isMobile) {
      setDetailDrawerOpen(true);
    }
  };

  const handleDelete = (entry: SummaryEntry) => {
    Modal.confirm({
      title: t('aiSummary.deleteTitle'),
      content: t('aiSummary.deleteConfirm'),
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteSummary(entry.id);
          setEntries((prev) => prev.filter((e) => e.id !== entry.id));
          if (selectedId === entry.id) {
            setSelectedId(null);
          }
        } catch {
          message.error(t('aiSummary.statusError'));
        }
      },
    });
  };

  const renderStatusTag = (status: SummaryEntry['status']) => {
    switch (status) {
      case 'analyzing':
        return (
          <Tag icon={<LoadingOutlined />} color="processing">
            {t('aiSummary.statusAnalyzing')}
          </Tag>
        );
      case 'completed':
        return <Tag color="success">{t('aiSummary.statusCompleted')}</Tag>;
      case 'error':
        return <Tag color="error">{t('aiSummary.statusError')}</Tag>;
    }
  };

  const renderHistoryList = () => {
    if (loading) {
      return (
        <div className="ai-summary-page__empty">
          <Spin size="large" />
        </div>
      );
    }

    if (entries.length === 0) {
      return (
        <div className="ai-summary-page__empty">
          <Empty description={t('aiSummary.emptyState')} />
        </div>
      );
    }

    return (
      <div className="ai-summary-page__list">
        {entries.map((entry) => {
          const showDelete = entry.status === 'completed' || entry.status === 'error';
          const isSelected = entry.id === selectedId;

          return (
            <div
              key={entry.id}
              className={`ai-summary-page__list-item ai-summary-page__list-item--clickable${isSelected ? ' ai-summary-page__list-item--selected' : ''}`}
              onClick={() => handleEntryClick(entry)}
            >
              <div className="ai-summary-page__list-item-info">
                <span className="ai-summary-page__list-item-range">
                  {entry.start_date} ~ {entry.end_date}
                </span>
                <div className="ai-summary-page__list-item-meta">
                  {renderStatusTag(entry.status)}
                  <span>{dayjs(entry.created_at).format('YYYY-MM-DD HH:mm')}</span>
                </div>
              </div>
              <div className="ai-summary-page__list-item-actions">
                {showDelete && (
                  <Button
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDelete(entry);
                    }}
                  />
                )}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  return (
    <div className="ai-summary-page">
      <div className="ai-summary-page__header">
        <h2>{t('aiSummary.title')}</h2>
        <Button type="primary" onClick={() => setDrawerOpen(true)}>
          {t('aiSummary.newAnalysis')}
        </Button>
      </div>

      <div className="ai-summary-page__body">
        <div className="ai-summary-page__left">
          {renderHistoryList()}
        </div>
        {!isMobile && (
          <div className="ai-summary-page__right">
            <SummaryDetailPanel summaryId={selectedId} />
          </div>
        )}
      </div>

      {isMobile && (
        <SummaryDetailDrawer
          open={detailDrawerOpen}
          onClose={() => setDetailDrawerOpen(false)}
          summaryId={selectedId}
        />
      )}

      <AnalysisDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onStartAnalysis={handleStartAnalysis}
      />
    </div>
  );
}
