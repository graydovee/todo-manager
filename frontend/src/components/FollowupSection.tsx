import { useState, useEffect, useRef, useCallback } from 'react';
import { Input, Button } from 'antd';
import { SendOutlined, LoadingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { listFollowups, createFollowup } from '../api/followups';
import { FollowupMessageBubble } from './FollowupMessageBubble';
import type { FollowupMessage, ContextMessage } from '../types/followup';
import './FollowupSection.css';

interface FollowupSectionProps {
  summaryId: number;
  summaryContent: string;
  /** Ref to the parent scrollable container for auto-scroll */
  scrollContainerRef?: React.RefObject<HTMLDivElement | null>;
}

export interface DisplayMessage {
  id: number;
  question: string;
  answer: string;
  isStreaming?: boolean;
  isError?: boolean;
}

// JSON payload from the SSE done event
interface FollowupDonePayload {
  followup_message_id: number;
  version_id: number;
  version_number: number;
}

const MAX_INPUT_LENGTH = 500;
const MAX_EXCHANGES = 20;

export function FollowupSection({ summaryId, summaryContent: _summaryContent, scrollContainerRef }: FollowupSectionProps) {
  const { t } = useTranslation();
  // summaryContent is available for future use in context construction
  void _summaryContent;
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [inputValue, setInputValue] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  // Track the last request params for retry
  const lastRequestRef = useRef<{
    question: string;
    contextMessages: ContextMessage[];
    targetId: number;
    type: 'send' | 'edit' | 'regenerate';
  } | null>(null);

  // Load existing followup messages on mount
  useEffect(() => {
    const loadMessages = async () => {
      setIsLoadingHistory(true);
      try {
        const data = await listFollowups(summaryId);
        const displayMessages: DisplayMessage[] = data
          .slice(0, MAX_EXCHANGES)
          .map((msg: FollowupMessage) => ({
            id: msg.id,
            question: msg.question,
            // Use the latest version content
            answer:
              msg.versions && msg.versions.length > 0
                ? msg.versions[msg.versions.length - 1].content
                : '',
          }));
        setMessages(displayMessages);
      } catch {
        // Silently fail on load - user can still ask new questions
      } finally {
        setIsLoadingHistory(false);
      }
    };

    void loadMessages();

    return () => {
      // Cleanup any ongoing stream on unmount
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [summaryId]);

  // Auto-scroll to bottom when messages change - scroll the parent container
  useEffect(() => {
    if (scrollContainerRef?.current) {
      scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight;
    } else if (messagesEndRef.current && typeof messagesEndRef.current.scrollIntoView === 'function') {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages, scrollContainerRef]);

  // Build context messages from messages preceding a given index
  const buildContextFromMessages = useCallback((msgs: DisplayMessage[], beforeIndex?: number): ContextMessage[] => {
    const slice = beforeIndex !== undefined ? msgs.slice(0, beforeIndex) : msgs;
    return slice
      .filter((msg) => msg.answer && !msg.isError)
      .slice(-MAX_EXCHANGES)
      .flatMap((msg) => [
        { role: 'user' as const, content: msg.question },
        { role: 'assistant' as const, content: msg.answer },
      ]);
  }, []);

  // Core SSE streaming logic - reused by send, edit, and regenerate
  const streamResponse = useCallback(async (
    question: string,
    contextMessages: ContextMessage[],
    targetId: number,
    requestType: 'send' | 'edit' | 'regenerate'
  ) => {
    // Store request params for retry
    lastRequestRef.current = { question, contextMessages, targetId, type: requestType };

    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    try {
      const response = await createFollowup(summaryId, question, contextMessages);

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error('No response body');
      }

      const decoder = new TextDecoder();
      let accumulatedContent = '';
      let buffer = '';
      let currentEventType: string | null = null;

      while (true) {
        if (abortController.signal.aborted) break;

        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // SSE events are separated by double newlines; process complete events
        const events = buffer.split('\n\n');
        buffer = events.pop() || '';

        for (const event of events) {
          const lines = event.split('\n');
          currentEventType = null;
          let eventData = '';

          for (const line of lines) {
            if (line.startsWith('event: ')) {
              currentEventType = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              eventData += line.slice(6);
            } else if (line === 'data:') {
              eventData += '';
            }
          }

          if (currentEventType === 'done') {
            // Parse the done event JSON payload with message/version IDs
            let realId = targetId;
            try {
              const payload: FollowupDonePayload = JSON.parse(eventData);
              if (payload.followup_message_id) {
                realId = payload.followup_message_id;
              }
            } catch {
              // If JSON parsing fails, keep the target ID
            }
            // Finalize the message and update with the real persisted ID
            setMessages((prev) =>
              prev.map((msg) =>
                msg.id === targetId
                  ? { ...msg, id: realId, isStreaming: false }
                  : msg
              )
            );
          } else if (currentEventType === 'error') {
            // Display error message from the SSE error event
            const errorMsg = eventData || t('aiSummary.followup.error');
            setMessages((prev) =>
              prev.map((msg) =>
                msg.id === targetId
                  ? { ...msg, answer: errorMsg, isStreaming: false, isError: true }
                  : msg
              )
            );
          } else {
            // Default data event - accumulate content chunks
            if (eventData) {
              accumulatedContent += eventData;
              setMessages((prev) =>
                prev.map((msg) =>
                  msg.id === targetId
                    ? { ...msg, answer: accumulatedContent, isStreaming: true }
                    : msg
                )
              );
            }
          }
        }
      }

      // Finalize if stream ended without explicit done event
      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === targetId && msg.isStreaming
            ? { ...msg, isStreaming: false }
            : msg
        )
      );
    } catch {
      // Network or other error
      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === targetId
            ? {
                ...msg,
                answer: t('aiSummary.followup.networkError'),
                isStreaming: false,
                isError: true,
              }
            : msg
        )
      );
    } finally {
      setIsStreaming(false);
      abortControllerRef.current = null;
    }
  }, [summaryId, t]);

  // Handle sending a new followup question
  const handleSend = useCallback(async () => {
    const question = inputValue.trim();
    if (!question || isStreaming) return;

    const contextMessages = buildContextFromMessages(messages);

    // Add user message immediately with a temporary ID
    const tempId = Date.now();
    const newMessage: DisplayMessage = {
      id: tempId,
      question,
      answer: '',
      isStreaming: true,
    };

    setMessages((prev) => [...prev.slice(-(MAX_EXCHANGES - 1)), newMessage]);
    setInputValue('');
    setIsStreaming(true);

    await streamResponse(question, contextMessages, tempId, 'send');
  }, [inputValue, isStreaming, messages, buildContextFromMessages, streamResponse]);

  // Handle editing a question at a given index
  const handleEdit = useCallback(async (index: number, newQuestion: string) => {
    if (isStreaming) return;

    // Build context from messages BEFORE the edited one
    const contextMessages = buildContextFromMessages(messages, index);

    // Truncate messages: keep only messages before the edited one,
    // then add the edited question as a new streaming message
    const tempId = Date.now();
    const editedMessage: DisplayMessage = {
      id: tempId,
      question: newQuestion,
      answer: '',
      isStreaming: true,
    };

    setMessages((prev) => [...prev.slice(0, index), editedMessage]);
    setIsStreaming(true);

    await streamResponse(newQuestion, contextMessages, tempId, 'edit');
  }, [isStreaming, messages, buildContextFromMessages, streamResponse]);

  // Handle regenerating a response for a message at a given index
  const handleRegenerate = useCallback(async (index: number) => {
    if (isStreaming) return;

    const targetMessage = messages[index];
    if (!targetMessage) return;

    // Build context from messages BEFORE this one (same as edit)
    const contextMessages = buildContextFromMessages(messages, index);

    // Set the current message to streaming state, clear its answer
    const tempId = Date.now();
    setMessages((prev) =>
      prev.map((msg, i) =>
        i === index
          ? { ...msg, id: tempId, answer: '', isStreaming: true, isError: false }
          : msg
      )
    );
    setIsStreaming(true);

    await streamResponse(targetMessage.question, contextMessages, tempId, 'regenerate');
  }, [isStreaming, messages, buildContextFromMessages, streamResponse]);

  // Handle retrying the last failed request
  const handleRetry = useCallback(async (index: number) => {
    if (isStreaming) return;

    const targetMessage = messages[index];
    if (!targetMessage) return;

    // Build context from messages BEFORE this one
    const contextMessages = buildContextFromMessages(messages, index);

    // Reset the error state and start streaming again
    const tempId = Date.now();
    setMessages((prev) =>
      prev.map((msg, i) =>
        i === index
          ? { ...msg, id: tempId, answer: '', isStreaming: true, isError: false }
          : msg
      )
    );
    setIsStreaming(true);

    await streamResponse(targetMessage.question, contextMessages, tempId, 'regenerate');
  }, [isStreaming, messages, buildContextFromMessages, streamResponse]);

  // Handle Enter key press
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      void handleSend();
    }
  };

  const isSendDisabled = !inputValue.trim() || isStreaming;

  return (
    <div className="followup-section" data-testid="followup-section">
      {/* Message list - rendered inline in the same scroll flow as summary content */}
      <div className="followup-section__messages">
        {isLoadingHistory && (
          <div className="followup-section__loading">
            <LoadingOutlined />
            <span>{t('aiSummary.followup.loading')}</span>
          </div>
        )}

        {messages.map((msg, index) => (
          <FollowupMessageBubble
            key={msg.id}
            question={msg.question}
            answer={msg.answer}
            isStreaming={msg.isStreaming}
            isError={msg.isError}
            onEdit={(newQuestion) => void handleEdit(index, newQuestion)}
            onRegenerate={() => void handleRegenerate(index)}
            onRetry={() => void handleRetry(index)}
          />
        ))}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area - fixed at bottom */}
      <div className="followup-section__input-area">
        <Input.TextArea
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={t('aiSummary.followup.inputPlaceholder')}
          maxLength={MAX_INPUT_LENGTH}
          autoSize={{ minRows: 1, maxRows: 4 }}
          disabled={isStreaming}
          data-testid="followup-input"
        />
        <Button
          type="primary"
          icon={isStreaming ? <LoadingOutlined /> : <SendOutlined />}
          onClick={handleSend}
          disabled={isSendDisabled}
          className="followup-section__send-btn"
          data-testid="followup-send-btn"
        >
          {t('aiSummary.followup.send')}
        </Button>
      </div>
    </div>
  );
}
