import { useState, useEffect, useRef, useCallback } from 'react';
import { Spin, Button } from 'antd';
import { ReloadOutlined, LoadingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ReactMarkdown from 'react-markdown';
import { getSummary, getStreamUrl } from '../api/summaries';
import type { SummaryEntry } from '../api/summaries';
import './SummaryDetailPanel.css';

interface Props {
  summaryId: number | null;
}

export function SummaryDetailPanel({ summaryId }: Props) {
  const { t } = useTranslation();

  const [summary, setSummary] = useState<SummaryEntry | null>(null);
  const [loading, setLoading] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const [content, setContent] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [connectionError, setConnectionError] = useState(false);

  const contentRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const userScrolledRef = useRef(false);

  // Auto-scroll to bottom unless user has scrolled up
  const scrollToBottom = useCallback(() => {
    if (!userScrolledRef.current && contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, []);

  // Detect user scroll
  const handleScroll = useCallback(() => {
    if (!contentRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = contentRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    userScrolledRef.current = !isAtBottom;
  }, []);

  // Scroll to bottom when content changes
  useEffect(() => {
    scrollToBottom();
  }, [content, scrollToBottom]);

  // Cleanup EventSource on unmount or when summaryId changes
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [summaryId]);

  // Establish SSE connection
  const connectSSE = useCallback((id: number) => {
    setStreaming(true);
    setError(null);
    setConnectionError(false);
    setContent('');
    userScrolledRef.current = false;

    const url = getStreamUrl(id);
    const es = new EventSource(url, { withCredentials: true });
    eventSourceRef.current = es;

    // Default message event: event.data already joins multi-line data: fields with \n per SSE spec
    es.onmessage = (event) => {
      setContent((prev) => prev + event.data);
    };

    // "done" event: stream complete
    es.addEventListener('done', () => {
      setStreaming(false);
      es.close();
      eventSourceRef.current = null;
    });

    // "error" custom event from server
    es.addEventListener('error', ((event: MessageEvent) => {
      const errorData = event.data || t('analysis.result.error');
      setError(errorData);
      setStreaming(false);
      es.close();
      eventSourceRef.current = null;
    }) as EventListener);

    // Native EventSource error (connection lost)
    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) {
        return;
      }
      setConnectionError(true);
      setStreaming(false);
      es.close();
      eventSourceRef.current = null;
    };
  }, [t]);

  // Fetch summary and determine display mode
  useEffect(() => {
    if (summaryId === null) {
      setSummary(null);
      setContent('');
      setError(null);
      setStreaming(false);
      setConnectionError(false);
      setLoading(false);
      return;
    }

    // Close any existing EventSource
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    setLoading(true);
    setContent('');
    setError(null);
    setStreaming(false);
    setConnectionError(false);
    userScrolledRef.current = false;

    const fetchAndConnect = async () => {
      try {
        const data = await getSummary(summaryId);
        setSummary(data);

        if (data.status === 'completed') {
          setContent(data.result_content || '');
        } else if (data.status === 'error') {
          setError(data.result_content || t('analysis.result.error'));
        } else if (data.status === 'analyzing') {
          connectSSE(summaryId);
        }
      } catch {
        setError(t('analysis.result.error'));
      } finally {
        setLoading(false);
      }
    };

    void fetchAndConnect();
  }, [summaryId, t, connectSSE]);

  const handleRetry = () => {
    if (summaryId === null) return;
    setConnectionError(false);
    connectSSE(summaryId);
  };

  // No summary selected - placeholder
  if (summaryId === null) {
    return (
      <div className="summary-detail-panel">
        <div className="summary-detail-panel__placeholder">
          {t('summaryDetail.selectSummary')}
        </div>
      </div>
    );
  }

  // Loading state
  if (loading) {
    return (
      <div className="summary-detail-panel">
        <div className="summary-detail-panel__loading">
          <Spin size="large" />
        </div>
      </div>
    );
  }

  // Error state (no content at all)
  if (error && !content) {
    return (
      <div className="summary-detail-panel">
        <div className="summary-detail-panel__error">
          <div className="summary-detail-panel__error-title">
            {t('analysis.result.error')}
          </div>
          <div className="summary-detail-panel__error-desc">{error}</div>
          {connectionError && (
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={handleRetry}
            >
              {t('analysis.result.retryButton')}
            </Button>
          )}
        </div>
      </div>
    );
  }

  // Connection error with partial content
  if (connectionError && content) {
    return (
      <div className="summary-detail-panel">
        <div
          className="summary-detail-panel__content"
          ref={contentRef}
          onScroll={handleScroll}
        >
          <ReactMarkdown>{content}</ReactMarkdown>
          <div className="summary-detail-panel__connection-error">
            <div className="summary-detail-panel__error-title">
              {t('analysis.result.connectionError')}
            </div>
            <div className="summary-detail-panel__error-desc">
              {t('analysis.result.connectionErrorDesc')}
            </div>
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={handleRetry}
            >
              {t('analysis.result.retryButton')}
            </Button>
          </div>
        </div>
      </div>
    );
  }

  // Completed with empty content
  if (summary?.status === 'completed' && !content) {
    return (
      <div className="summary-detail-panel">
        <div className="summary-detail-panel__placeholder">
          {t('summaryDetail.emptyContent')}
        </div>
      </div>
    );
  }

  // Main content display (streaming or completed)
  return (
    <div className="summary-detail-panel">
      <div
        className="summary-detail-panel__content"
        ref={contentRef}
        onScroll={handleScroll}
      >
        {content && (
          <>
            <ReactMarkdown>{content}</ReactMarkdown>
            {streaming && <span className="summary-detail-panel__cursor" />}
          </>
        )}

        {/* Streaming indicator when no content yet */}
        {streaming && !content && (
          <div className="summary-detail-panel__streaming-hint">
            <LoadingOutlined />
            <span>{t('analysis.result.streaming')}</span>
          </div>
        )}
      </div>
    </div>
  );
}
