import { useEffect, useMemo, useState, useRef, useCallback } from 'react';
import { Drawer, Form, Input, Select, DatePicker, Button, Space, Alert } from 'antd';
import { useTranslation } from 'react-i18next';
import { useCreateTodo, useUpdateTodo, useTodo } from '../hooks/useTodos';
import { listTodos } from '../api/todos';
import type { Todo, Category, TodoSummary } from '../types';
import type { Dayjs } from 'dayjs';

interface Props {
  open: boolean;
  todoId: number | null;
  onClose: () => void;
  defaultCategory?: Category;
  lockedPrerequisite?: TodoSummary;
  onCreated?: (todoId: number) => void;
}

interface TodoOption {
  id: number;
  code: string;
  title: string;
}

interface TodoFormValues {
  title: string;
  description?: string;
  category: Category;
  priority?: Todo['priority'];
  tags?: string[];
  due_at?: Dayjs;
  depends_on_ids?: number[];
}

function useTodoSearch() {
  const [results, setResults] = useState<TodoOption[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const initialLoaded = useRef(false);

  const search = useCallback((q: string) => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(async () => {
      try {
        const res = await listTodos({ q: q || undefined, page_size: 20, status: 'open,in_progress' });
        setResults(res.items.map((t: Todo) => ({ id: t.id, code: t.code, title: t.title })));
        if (!q) initialLoaded.current = true;
      } catch {
        setResults([]);
      }
    }, 300);
  }, []);

  const loadInitial = useCallback(() => {
    if (!initialLoaded.current) {
      search('');
    }
  }, [search]);

  useEffect(() => () => clearTimeout(timerRef.current), []);

  return { results, search, loadInitial };
}

export function TodoForm({ open, todoId, onClose, defaultCategory, lockedPrerequisite, onCreated }: Props) {
  const [form] = Form.useForm();
  const { t } = useTranslation();
  const createMutation = useCreateTodo();
  const updateMutation = useUpdateTodo();
  const { data: todo } = useTodo(todoId || 0);
  const depsSearch = useTodoSearch();

  const isEdit = !!todoId;
  const hasLockedPrerequisite = !!lockedPrerequisite;
  const depsOptions = useMemo<TodoOption[]>(
    () => todo?.depends_on?.map((dep) => ({ id: dep.id, code: dep.code, title: dep.title })) || [],
    [todo],
  );

  useEffect(() => {
    if (open) {
      if (todo) {
        form.setFieldsValue({
          title: todo.title,
          description: todo.description,
          category: todo.category,
          priority: todo.priority,
          tags: todo.tags,
          depends_on_ids: todo.depends_on?.map((d) => d.id) || [],
        });
      } else {
        form.resetFields();
        if (defaultCategory) form.setFieldValue('category', defaultCategory);
      }
    }
  }, [open, todo, defaultCategory, lockedPrerequisite, form]);

  const handleSubmit = async (values: TodoFormValues) => {
    const input = {
      ...values,
      tags: values.tags || [],
      depends_on_ids: values.depends_on_ids || [],
      due_at: values.due_at?.toISOString(),
    };

    if (isEdit && todoId) {
      await updateMutation.mutateAsync({ id: todoId, input });
    } else {
      const result = await createMutation.mutateAsync(input);
      if (onCreated && result) {
        onCreated(result.id);
      }
    }
    onClose();
  };

  const toOption = (t: TodoOption) => ({ value: t.id, label: `${t.code} - ${t.title}` });

  return (
    <Drawer
      title={isEdit ? t('todo.editTodo') : t('todo.newTodo')}
      open={open}
      onClose={onClose}
      width={480}
      destroyOnClose
    >
      <Form form={form} layout="vertical" onFinish={handleSubmit} initialValues={{ priority: 'p2' }}>
        {hasLockedPrerequisite && (
          <Alert
            type="info"
            showIcon={false}
            style={{ marginBottom: 16 }}
            message={`${t('detail.addPrerequisiteFor')}: ${lockedPrerequisite.code} - ${lockedPrerequisite.title}`}
          />
        )}

        <Form.Item name="title" label={t('todo.titleLabel')} rules={[{ required: true, message: t('todo.titleRequired') }]}>
          <Input placeholder={t('todo.titlePlaceholder')} />
        </Form.Item>

        <Form.Item name="description" label={t('todo.description')}>
          <Input.TextArea rows={3} placeholder={t('todo.description')} />
        </Form.Item>

        <Form.Item name="category" label={t('todo.category')} rules={[{ required: true, message: t('todo.categoryRequired') }]}>
          <Select disabled={isEdit} options={[
            { value: 'bug', label: t('todo.bug') },
            { value: 'feature', label: t('todo.feature') },
            { value: 'task', label: t('todo.task') },
          ]} />
        </Form.Item>

        <Form.Item name="priority" label={t('todo.priority')}>
          <Select options={[
            { value: 'p0', label: t('todo.p0') },
            { value: 'p1', label: t('todo.p1') },
            { value: 'p2', label: t('todo.p2') },
            { value: 'p3', label: t('todo.p3') },
          ]} />
        </Form.Item>

        <Form.Item name="tags" label={t('todo.tags')}>
          <Select mode="tags" placeholder={t('todo.addTags')} tokenSeparators={[',']} />
        </Form.Item>

        <Form.Item name="due_at" label={t('todo.dueDate')}>
          <DatePicker style={{ width: '100%' }} showTime />
        </Form.Item>

        <Form.Item name="depends_on_ids" label={t('todo.dependsOn')}>
          <Select
            mode="multiple"
            showSearch
            allowClear
            filterOption={false}
            placeholder={t('todo.enterTodoIds')}
            onSearch={(q) => {
              depsSearch.search(q);
            }}
            onDropdownVisibleChange={(visible) => {
              if (visible) depsSearch.loadInitial();
            }}
            options={[...depsOptions, ...depsSearch.results]
              .filter((t, i, arr) => arr.findIndex((x) => x.id === t.id) === i)
              .map(toOption)}
          />
        </Form.Item>

        <Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createMutation.isPending || updateMutation.isPending}>
              {isEdit ? t('todo.update') : t('todo.create')}
            </Button>
            <Button onClick={onClose}>{t('common.cancel')}</Button>
          </Space>
        </Form.Item>
      </Form>
    </Drawer>
  );
}
