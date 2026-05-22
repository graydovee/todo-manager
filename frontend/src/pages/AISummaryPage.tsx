import { useState, useEffect, useCallback, useRef } from 'react';
import { Button, DatePicker, Drawer, Empty, Modal, Spin, Tag, message } from 'antd';
import { DeleteOutlined, CloseOutlined, LoadingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ReactMarkdown from 'react-markdown';
import { createSummary, deleteSummary, getSummary, listSummaries } from '../api/summaries';
import type { SummaryEntry } from '../api/summaries';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import './AISummaryPage.css';

const { RangePicker } = DatePicker;

const POLL_INTERVAL = 2000;
const POLL_TIMEOUT = 60000;

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
  const [analyzing, setAnalyzing] = useState(false);
  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs] | null>(null);
  const [selectedEntry, setSelectedEntry] = useState<SummaryEntry | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollingStartRef = useRef<number>(0);

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

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
      }
    };
  }, []);

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  const startPolling = useCallback((id: number) => {
    stopPolling();
    pollingStartRef.current = Date.now();

    pollingRef.current = setInterval(async () => {
      const elapsed = Date.now() - pollingStartRef.current;
      if (elapsed >= POLL_TIMEOUT) {
        stopPolling();
        setAnalyzing(false);
        // Update the entry to error status locally
        setEntries((prev) =>
          prev.map((e) =>
            e.id === id ? { ...e, status: 'error' as const } : e
          )
        );
        return;
      }

      try {
        const updated = await getSummary(id);
        if (updated.status !== 'analyzing') {
          stopPolling();
          setAnalyzing(false);
          setEntries((prev) =>
            prev.map((e) => (e.id === id ? updated : e))
          );
        }
      } catch {
        // Polling error - continue trying
      }
    }, POLL_INTERVAL);
  }, [stopPolling]);

  const handleAnalyze = async () => {
    if (!dateRange) return;

    const startDate = dateRange[0].format('YYYY-MM-DD');
    const endDate = dateRange[1].format('YYYY-MM-DD');

    setAnalyzing(true);
    try {
      const entry = await createSummary(startDate, endDate);
      setEntries((prev) => [entry, ...prev]);
      startPolling(entry.id);
    } catch {
      setAnalyzing(false);
      message.error(t('aiSummary.statusError'));
    }
  };

  const handleEntryClick = (entry: SummaryEntry) => {
    if (entry.status !== 'completed') return;
    setSelectedEntry(entry);
    if (isMobile) {
      setDrawerOpen(true);
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
          // Close detail if the deleted entry was selected
          if (selectedEntry?.id === entry.id) {
            setSelectedEntry(null);
            setDrawerOpen(false);
          }
        } catch {
          message.error(t('aiSummary.statusError'));
        }
      },
    });
  };

  const handleCloseDetail = () => {
    setSelectedEntry(null);
    setDrawerOpen(false);
  };

  const disableFutureDates = (current: Dayjs) => {
    return current && current.isAfter(dayjs().endOf('day'));
  };

  const rangePresets: { label: string; value: [Dayjs, Dayjs] }[] = [
    { label: t('aiSummary.presetToday'), value: [dayjs(), dayjs()] },
    { label: t('aiSummary.presetLast3Days'), value: [dayjs().subtract(2, 'day'), dayjs()] },
    { label: t('aiSummary.presetLast7Days'), value: [dayjs().subtract(6, 'day'), dayjs()] },
    { label: t('aiSummary.presetLast30Days'), value: [dayjs().subtract(29, 'day'), dayjs()] },
    { label: t('aiSummary.presetThisMonth'), value: [dayjs().startOf('month'), dayjs()] },
    { label: t('aiSummary.presetLastMonth'), value: [dayjs().subtract(1, 'month').startOf('month'), dayjs().subtract(1, 'month').endOf('month')] },
  ];

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
          const isClickable = entry.status === 'completed';
          const isSelected = selectedEntry?.id === entry.id;
          const showDelete = entry.status === 'completed' || entry.status === 'error';

          return (
            <div
              key={entry.id}
              className={[
                'ai-summary-page__list-item',
                isClickable ? 'ai-summary-page__list-item--clickable' : '',
                isSelected ? 'ai-summary-page__list-item--selected' : '',
              ]
                .filter(Boolean)
                .join(' ')}
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

  const renderDetailContent = () => {
    if (!selectedEntry) {
      return (
        <div className="ai-summary-page__empty">
          <Empty description={t('aiSummary.selectRange')} />
        </div>
      );
    }

    return (
      <>
        <div className="ai-summary-page__detail-header">
          <h3>
            {selectedEntry.start_date} ~ {selectedEntry.end_date}
          </h3>
          <Button
            type="text"
            icon={<CloseOutlined />}
            onClick={handleCloseDetail}
          />
        </div>
        <div className="ai-summary-page__detail-content">
          <ReactMarkdown>{selectedEntry.result_content || ''}</ReactMarkdown>
        </div>
      </>
    );
  };

  return (
    <div className="ai-summary-page">
      <div className="ai-summary-page__header">
        <h2>{t('aiSummary.title')}</h2>
      </div>

      <div className="ai-summary-page__body">
        <div className="ai-summary-page__left">
          <div className="ai-summary-page__controls">
            <RangePicker
              value={dateRange}
              onChange={(dates) => setDateRange(dates as [Dayjs, Dayjs] | null)}
              disabledDate={disableFutureDates}
              placeholder={[t('aiSummary.startDate'), t('aiSummary.endDate')]}
              presets={rangePresets}
            />
            <Button
              type="primary"
              onClick={handleAnalyze}
              disabled={!dateRange || analyzing}
              loading={analyzing}
            >
              {t('aiSummary.analyzeButton')}
            </Button>
          </div>

          {renderHistoryList()}
        </div>

        {!isMobile && (
          <div className="ai-summary-page__right">
            {renderDetailContent()}
          </div>
        )}
      </div>

      {isMobile && (
        <Drawer
          title={selectedEntry ? `${selectedEntry.start_date} ~ ${selectedEntry.end_date}` : ''}
          placement="right"
          width="85%"
          open={drawerOpen && !!selectedEntry}
          onClose={handleCloseDetail}
          className="ai-summary-mobile-drawer"
          destroyOnClose={false}
        >
          {selectedEntry && (
            <div className="ai-summary-page__detail-content">
              <ReactMarkdown>{selectedEntry.result_content || ''}</ReactMarkdown>
            </div>
          )}
        </Drawer>
      )}
    </div>
  );
}
