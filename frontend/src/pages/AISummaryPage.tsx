import { useState, useEffect, useCallback } from 'react';
import { Button, Empty, Modal, Spin, Tag, message } from 'antd';
import { DeleteOutlined, LoadingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { createSummaryWithTodos, deleteSummary, listSummaries } from '../api/summaries';
import type { SummaryEntry } from '../api/summaries';
import { AnalysisDrawer } from '../components/AnalysisDrawer';
import dayjs from 'dayjs';
import './AISummaryPage.css';

export function AISummaryPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [entries, setEntries] = useState<SummaryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [drawerOpen, setDrawerOpen] = useState(false);

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

  const handleStartAnalysis = async (startDate: string, endDate: string, todoIds: number[]) => {
    try {
      const entry = await createSummaryWithTodos(startDate, endDate, todoIds);
      navigate(`/ai-summary/${entry.id}`);
    } catch {
      message.error(t('aiSummary.statusError'));
    }
  };

  const handleEntryClick = (entry: SummaryEntry) => {
    navigate(`/ai-summary/${entry.id}`);
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

          return (
            <div
              key={entry.id}
              className="ai-summary-page__list-item ai-summary-page__list-item--clickable"
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
        {renderHistoryList()}
      </div>

      <AnalysisDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onStartAnalysis={handleStartAnalysis}
      />
    </div>
  );
}
