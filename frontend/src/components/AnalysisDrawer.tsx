import { useState, useEffect, useCallback } from 'react';
import { Drawer, DatePicker, Button, Checkbox, List, Spin, Empty, Tag, Select, Input } from 'antd';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { fetchTodosByDateRange } from '../api/todos';
import type { TodoByDateRangeItem } from '../api/todos';
import './AnalysisDrawer.css';

const { RangePicker } = DatePicker;

interface AnalysisDrawerProps {
  open: boolean;
  onClose: () => void;
  onStartAnalysis: (startDate: string, endDate: string, todoIds: number[], language: string, customPrompt?: string) => void;
}

// Map frontend language value to backend language value
function mapLanguageToBackend(lang: string): string {
  switch (lang) {
    case 'zh':
      return 'Chinese';
    case 'en':
      return 'English';
    default:
      return '';
  }
}

// Derive initial language from current i18n locale
function getDefaultLanguage(locale: string): string {
  if (locale === 'zh') return 'zh';
  if (locale === 'en') return 'en';
  return '';
}

export function AnalysisDrawer({ open, onClose, onStartAnalysis }: AnalysisDrawerProps) {
  const { t, i18n } = useTranslation();

  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs] | null>(null);
  const [todos, setTodos] = useState<TodoByDateRangeItem[]>([]);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [loading, setLoading] = useState(false);
  const [language, setLanguage] = useState<string>(() => getDefaultLanguage(i18n.language));
  const [customPrompt, setCustomPrompt] = useState('');

  // Fetch todos when date range changes
  const fetchTodos = useCallback(async (start: string, end: string) => {
    setLoading(true);
    try {
      const data = await fetchTodosByDateRange(start, end);
      setTodos(data);
      // Select all by default
      setSelectedIds(data.map((item) => item.id));
    } catch {
      setTodos([]);
      setSelectedIds([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (dateRange) {
      const startDate = dateRange[0].format('YYYY-MM-DD');
      const endDate = dateRange[1].format('YYYY-MM-DD');
      void fetchTodos(startDate, endDate);
    } else {
      setTodos([]);
      setSelectedIds([]);
    }
  }, [dateRange, fetchTodos]);

  // Reset state when drawer closes
  useEffect(() => {
    if (!open) {
      setDateRange(null);
      setTodos([]);
      setSelectedIds([]);
      setLanguage(getDefaultLanguage(i18n.language));
      setCustomPrompt('');
    }
  }, [open, i18n.language]);

  const handleDateRangeChange = (dates: [Dayjs, Dayjs] | null) => {
    setDateRange(dates);
  };

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(todos.map((item) => item.id));
    } else {
      setSelectedIds([]);
    }
  };

  const handleToggleItem = (id: number, checked: boolean) => {
    if (checked) {
      setSelectedIds((prev) => [...prev, id]);
    } else {
      setSelectedIds((prev) => prev.filter((i) => i !== id));
    }
  };

  const handleStartAnalysis = () => {
    if (!dateRange || selectedIds.length === 0) return;
    const startDate = dateRange[0].format('YYYY-MM-DD');
    const endDate = dateRange[1].format('YYYY-MM-DD');
    const trimmedPrompt = customPrompt.trim();
    onStartAnalysis(startDate, endDate, selectedIds, mapLanguageToBackend(language), trimmedPrompt || undefined);
    onClose();
  };

  const disableFutureDates = (current: Dayjs) => {
    return current && current.isAfter(dayjs().endOf('day'));
  };

  const rangePresets: { label: string; value: [Dayjs, Dayjs] }[] = [
    { label: t('aiSummary.drawer.presetLast7Days'), value: [dayjs().subtract(6, 'day'), dayjs()] },
    { label: t('aiSummary.drawer.presetLast30Days'), value: [dayjs().subtract(29, 'day'), dayjs()] },
    { label: t('aiSummary.drawer.presetLast90Days'), value: [dayjs().subtract(89, 'day'), dayjs()] },
  ];

  const isAllSelected = todos.length > 0 && selectedIds.length === todos.length;
  const isIndeterminate = selectedIds.length > 0 && selectedIds.length < todos.length;

  const renderStatusTag = (status: string) => {
    switch (status) {
      case 'completed':
        return <Tag color="success">{t('todo.completed')}</Tag>;
      case 'in_progress':
        return <Tag color="processing">{t('todo.inProgress')}</Tag>;
      default:
        return <Tag>{t('todo.open')}</Tag>;
    }
  };

  const renderTodoList = () => {
    if (!dateRange) {
      return (
        <div className="analysis-drawer__empty">
          <Empty description={t('aiSummary.drawer.selectDateRange')} />
        </div>
      );
    }

    if (loading) {
      return (
        <div className="analysis-drawer__loading">
          <Spin />
        </div>
      );
    }

    if (todos.length === 0) {
      return (
        <div className="analysis-drawer__empty">
          <Empty description={t('aiSummary.drawer.emptyTodos')} />
        </div>
      );
    }

    return (
      <div className="analysis-drawer__todo-list">
        <div className="analysis-drawer__select-all">
          <Checkbox
            checked={isAllSelected}
            indeterminate={isIndeterminate}
            onChange={(e) => handleSelectAll(e.target.checked)}
          >
            {t('aiSummary.drawer.selectAll')}
          </Checkbox>
        </div>
        <List
          dataSource={todos}
          renderItem={(item) => (
            <List.Item className="analysis-drawer__todo-item">
              <Checkbox
                checked={selectedIds.includes(item.id)}
                onChange={(e) => handleToggleItem(item.id, e.target.checked)}
              >
                <span className="analysis-drawer__todo-code">{item.code}</span>
                <span className="analysis-drawer__todo-title">{item.title}</span>
              </Checkbox>
              {renderStatusTag(item.status)}
            </List.Item>
          )}
        />
      </div>
    );
  };

  return (
    <Drawer
      title={t('aiSummary.drawer.title')}
      placement="right"
      open={open}
      onClose={onClose}
      width={480}
      className="analysis-drawer"
      footer={
        <div className="analysis-drawer__footer">
          <Button onClick={onClose}>{t('common.cancel')}</Button>
          <Button
            type="primary"
            disabled={selectedIds.length === 0}
            onClick={handleStartAnalysis}
          >
            {t('aiSummary.drawer.startButton')}
          </Button>
        </div>
      }
    >
      <div className="analysis-drawer__content">
        <div className="analysis-drawer__date-picker">
          <RangePicker
            value={dateRange}
            onChange={(dates) => handleDateRangeChange(dates as [Dayjs, Dayjs] | null)}
            disabledDate={disableFutureDates}
            placeholder={[t('aiSummary.startDate'), t('aiSummary.endDate')]}
            presets={rangePresets}
            style={{ width: '100%' }}
          />
        </div>
        <div className="analysis-drawer__language-selector">
          <label className="analysis-drawer__language-label">
            {t('aiSummary.languageLabel')}
          </label>
          <Select
            value={language}
            onChange={(value: string) => setLanguage(value)}
            style={{ width: '100%' }}
            options={[
              { value: '', label: t('aiSummary.languageAuto') },
              { value: 'zh', label: t('aiSummary.languageChinese') },
              { value: 'en', label: t('aiSummary.languageEnglish') },
            ]}
          />
        </div>
        <div className="analysis-drawer__custom-prompt">
          <Input.TextArea
            value={customPrompt}
            onChange={(e) => setCustomPrompt(e.target.value)}
            placeholder={t('aiSummary.customPrompt.placeholder')}
            maxLength={500}
            showCount
            autoSize={{ minRows: 3 }}
          />
        </div>
        {renderTodoList()}
      </div>
    </Drawer>
  );
}
