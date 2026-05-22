import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Button, Spin } from 'antd';
import { ArrowLeftOutlined, ReloadOutlined, LoadingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ReactMarkdown from 'react-markdown';
import { getSummary, getStreamUrl } from '../api/summaries';
import type { SummaryEntry } from '../api/summaries';
import './AISummaryResultPage.css';

export function AISummaryResultPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [summary, setSummary] = useState<SummaryEntry | null>(null);
  const [loading, setLoading] = useState(true);
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
    // If user is near the bottom (within 50px), consider it auto-scroll territory
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    userScrolledRef.current = !isAtBottom;
  }, []);

  // Scroll to bottom when content changes
  useEffect(() => {
    scrollToBottom();
  }, [content, scrollToBottom]);

  // Cleanup EventSource on unmount
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, []);

  // Establish SSE connection
  const connectSSE = useCallback(() => {
    if (!id) return;

    const numericId = Number(id);
    if (isNaN(numericId)) return;

    setStreaming(true);
    setError(null);
    setConnectionError(false);
    setContent('');
    userScrolledRef.current = false;

    const url = getStreamUrl(numericId);
    const es = new EventSource(url, { withCredentials: true });
    eventSourceRef.current = es;

    // Default message event: append chunk data to content
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
      // Only handle if we haven't already received a custom error event
      if (es.readyState === EventSource.CLOSED) {
        // Connection was closed by server (normal after done/error event)
        return;
      }
      // Unexpected disconnect
      setConnectionError(true);
      setStreaming(false);
      es.close();
      eventSourceRef.current = null;
    };
  }, [id, t]);

  // Fetch summary on mount to determine status
  useEffect(() => {
    if (!id) return;

    const numericId = Number(id);
    if (isNaN(numericId)) {
      setLoading(false);
      setError(t('analysis.result.error'));
      return;
    }

    const fetchAndConnect = async () => {
      try {
        const data = await getSummary(numericId);
        setSummary(data);

        if (data.status === 'completed') {
          // Display stored content directly
          setContent(data.result_content || '');
        } else if (data.status === 'error') {
          // Display error
          setError(data.result_content || t('analysis.result.error'));
        } else if (data.status === 'analyzing') {
          // Establish SSE connection
          connectSSE();
        }
      } catch {
        setError(t('analysis.result.error'));
      } finally {
        setLoading(false);
      }
    };

    void fetchAndConnect();
  }, [id, t, connectSSE]);

  const handleRetry = () => {
    setConnectionError(false);
    connectSSE();
  };

  const handleBack = () => {
    navigate('/ai-summary');
  };

  // Loading state
  if (loading) {
    return (
      <div className="ai-result-page">
        <div className="ai-result-page__loading">
          <Spin size="large" tip={t('analysis.result.loading')} />
        </div>
      </div>
    );
  }

  return (
    <div className="ai-result-page">
      <div className="ai-result-page__header">
        <Button
          type="text"
          icon={<ArrowLeftOutlined />}
          onClick={handleBack}
        />
        <h2>
          {t('analysis.result.title')}
          {summary && ` — ${summary.start_date} ~ ${summary.end_date}`}
        </h2>
      </div>

      <div
        className="ai-result-page__content"
        ref={contentRef}
        onScroll={handleScroll}
      >
        {/* Error state */}
        {error && !content && (
          <div className="ai-result-page__error">
            <div className="ai-result-page__error-title">
              {t('analysis.result.error')}
            </div>
            <div className="ai-result-page__error-desc">{error}</div>
          </div>
        )}

        {/* Connection error with retry */}
        {connectionError && (
          <div className="ai-result-page__error">
            <div className="ai-result-page__error-title">
              {t('analysis.result.connectionError')}
            </div>
            <div className="ai-result-page__error-desc">
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
        )}

        {/* Markdown content */}
        {content && (
          <>
            <ReactMarkdown>{content}</ReactMarkdown>
            {streaming && <span className="ai-result-page__cursor" />}
          </>
        )}

        {/* Streaming indicator */}
        {streaming && !content && (
          <div className="ai-result-page__streaming-hint">
            <LoadingOutlined />
            <span>{t('analysis.result.streaming')}</span>
          </div>
        )}
      </div>
    </div>
  );
}
