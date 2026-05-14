import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Card, Drawer, Empty, Segmented, Select, Space, Spin, Switch, Tag, Typography } from 'antd';
import { ReloadOutlined, RadarChartOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import {
  ReactFlow,
  Background,
  Controls,
  MarkerType,
  MiniMap,
  Position,
  ReactFlowProvider,
  type Edge,
  type Node,
  type NodeProps,
  type ReactFlowInstance,
  type NodeTypes,
} from '@xyflow/react';
import ELK, { type ElkNode } from 'elkjs/lib/elk.bundled.js';
import '@xyflow/react/dist/style.css';
import './TodoGraphPage.css';
import { useTodoGraph } from '../hooks/useTodos';
import { TodoDetailPanel } from '../components/TodoDetailPanel';
import { TodoForm } from '../components/TodoForm';
import { getTodo, updateTodo } from '../api/todos';
import type { TodoGraphComponent, TodoGraphNode, TodoSummary } from '../types';

const { Title, Paragraph, Text } = Typography;

const elk = new ELK();

const NODE_WIDTH = 268;
const NODE_HEIGHT = 170;
const COMPONENT_GAP_X = 320;
const COMPONENT_GAP_Y = 260;

interface GraphNodeData {
  [key: string]: unknown;
  todo: TodoGraphNode;
  selected: boolean;
  neighbor: boolean;
  statusLabel: string;
  dueLabel: string;
  prereqLabel: string;
  dependentLabel: string;
  rootLabel: string;
  onSelect: (todoId: number) => void;
}

type GraphFlowNode = Node<GraphNodeData, 'todoCard'>;
type GraphFlowEdge = Edge;

function TodoGraphCardNode({ data }: NodeProps<GraphFlowNode>) {
  const dueAt = data.todo.due_at ? new Date(data.todo.due_at).toLocaleDateString() : null;

  return (
    <div
      className={[
        'todo-graph-node',
        data.selected ? 'is-selected' : '',
        data.neighbor ? 'is-neighbor' : '',
      ].filter(Boolean).join(' ')}
      onClick={() => data.onSelect(data.todo.id)}
    >
      <div className="todo-graph-node__top">
        <div className="todo-graph-node__code">{data.todo.code}</div>
        {data.todo.is_component_root && <div className="todo-graph-node__root">{data.rootLabel}</div>}
      </div>
      <div className="todo-graph-node__title">{data.todo.title}</div>
      <div className="todo-graph-node__meta">
        <span className={`todo-graph-node__chip status-${data.todo.status}`}>{data.statusLabel}</span>
        <span className={`todo-graph-node__chip category-${data.todo.category}`}>{data.todo.category.toUpperCase()}</span>
        <span className="todo-graph-node__chip">{data.todo.priority.toUpperCase()}</span>
      </div>
      <div className="todo-graph-node__footer">
        <div className="todo-graph-node__metric">
          <span className="todo-graph-node__metric-label">{data.prereqLabel}</span>
          <span className="todo-graph-node__metric-value">{data.todo.prerequisite_count}</span>
        </div>
        <div className="todo-graph-node__metric">
          <span className="todo-graph-node__metric-label">{data.dependentLabel}</span>
          <span className="todo-graph-node__metric-value">{data.todo.dependent_count}</span>
        </div>
        <div className="todo-graph-node__metric">
          <span className="todo-graph-node__metric-label">{data.dueLabel}</span>
          <span className="todo-graph-node__metric-value">{dueAt || '-'}</span>
        </div>
      </div>
    </div>
  );
}

const nodeTypes: NodeTypes = {
  todoCard: TodoGraphCardNode,
};

async function layoutComponent(
  component: TodoGraphComponent,
  nodes: TodoGraphNode[],
  edges: GraphFlowEdge[],
): Promise<{ nodes: GraphFlowNode[]; edges: GraphFlowEdge[]; width: number; height: number }> {
  const nodeIds = new Set(component.node_ids);
  const componentNodes = nodes.filter((node) => nodeIds.has(node.id));
  const componentEdges = edges.filter((edge) => nodeIds.has(Number(edge.source)) && nodeIds.has(Number(edge.target)));

  const elkGraph: ElkNode = {
    id: component.id,
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'DOWN',
      'elk.edgeRouting': 'SPLINES',
      'elk.layered.spacing.nodeNodeBetweenLayers': '90',
      'elk.spacing.nodeNode': '54',
      'elk.padding': '[top=24,left=24,bottom=24,right=24]',
      'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
    },
    children: componentNodes.map((node) => ({
      id: String(node.id),
      width: NODE_WIDTH,
      height: NODE_HEIGHT,
    })),
    edges: componentEdges.map((edge) => ({
      id: edge.id,
      sources: [String(edge.source)],
      targets: [String(edge.target)],
    })),
  };

  const layout = await elk.layout(elkGraph);
  const positioned = new Map<string, { x: number; y: number }>();
  for (const child of layout.children || []) {
    positioned.set(child.id, { x: child.x || 0, y: child.y || 0 });
  }

  const flowNodes: GraphFlowNode[] = componentNodes.map((node) => {
    const pos = positioned.get(String(node.id)) || { x: 0, y: 0 };
    return {
      id: String(node.id),
      type: 'todoCard',
      position: pos,
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top,
      data: {
        todo: node,
        selected: false,
        neighbor: false,
        statusLabel: '',
        dueLabel: '',
        prereqLabel: '',
        dependentLabel: '',
        rootLabel: '',
        onSelect: () => {},
      },
      draggable: false,
    };
  });

  const width = Math.max(...flowNodes.map((node) => node.position.x + NODE_WIDTH), NODE_WIDTH) + 64;
  const height = Math.max(...flowNodes.map((node) => node.position.y + NODE_HEIGHT), NODE_HEIGHT) + 64;

  return { nodes: flowNodes, edges: componentEdges, width, height };
}

function TodoGraphPageInner() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { data, isLoading, refetch } = useTodoGraph();
  const [selectedTodoId, setSelectedTodoId] = useState<number | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [prerequisiteForId, setPrerequisiteForId] = useState<number | undefined>(undefined);
  const flowRef = useRef<ReactFlowInstance<GraphFlowNode, GraphFlowEdge> | null>(null);

  const focusTodoId = searchParams.get('focus') ? Number(searchParams.get('focus')) : null;
  const showCompletedComponents = searchParams.get('show_completed_components') === '1';
  const hideCompletedNodes = searchParams.get('hide_completed_nodes') === '1';

  const focusComponent = useMemo(() => {
    if (!data || !focusTodoId) return null;
    return data.components.find((component) => component.node_ids.includes(focusTodoId)) || null;
  }, [data, focusTodoId]);

  const visibleGraph = useMemo(() => {
    if (!data) {
      return { nodes: [] as TodoGraphNode[], edges: [] as GraphFlowEdge[], components: [] as TodoGraphComponent[] };
    }

    const focusedCompleted = !!focusComponent?.all_completed;
    let components = focusComponent ? [focusComponent] : data.components;

    if (!focusComponent && !showCompletedComponents) {
      components = components.filter((component) => !component.all_completed);
    }

    const visibleComponentIds = new Set(components.map((component) => component.id));
    let nodes = data.nodes.filter((node) => visibleComponentIds.has(node.component_id));
    let edges = data.edges
      .filter((edge) => {
        const source = data.nodes.find((node) => node.id === edge.source_id);
        const target = data.nodes.find((node) => node.id === edge.target_id);
        return !!source && !!target && visibleComponentIds.has(source.component_id) && visibleComponentIds.has(target.component_id);
      })
      .map<GraphFlowEdge>((edge) => ({
        id: `${edge.source_id}-${edge.target_id}`,
        source: String(edge.source_id),
        target: String(edge.target_id),
        type: 'smoothstep',
        markerEnd: { type: MarkerType.ArrowClosed, width: 18, height: 18, color: 'rgba(30, 64, 175, 0.55)' },
        style: { stroke: 'rgba(30, 64, 175, 0.38)', strokeWidth: 2.2 },
        animated: false,
      }));

    if (hideCompletedNodes && !focusedCompleted) {
      const visibleNodeIds = new Set(nodes.filter((node) => node.status !== 'completed').map((node) => node.id));
      nodes = nodes.filter((node) => visibleNodeIds.has(node.id));
      edges = edges.filter((edge) => visibleNodeIds.has(Number(edge.source)) && visibleNodeIds.has(Number(edge.target)));
      const nodeIdSet = new Set(nodes.map((node) => node.id));
      components = components.filter((component) => component.node_ids.some((nodeId) => nodeIdSet.has(nodeId)));
    }

    return { nodes, edges, components };
  }, [data, focusComponent, hideCompletedNodes, showCompletedComponents]);

  const visibleNodeIds = useMemo(() => new Set(visibleGraph.nodes.map((node) => node.id)), [visibleGraph.nodes]);

  const neighborIds = useMemo(() => {
    if (!selectedTodoId) return new Set<number>();
    const ids = new Set<number>();
    for (const edge of visibleGraph.edges) {
      const source = Number(edge.source);
      const target = Number(edge.target);
      if (source === selectedTodoId) ids.add(target);
      if (target === selectedTodoId) ids.add(source);
    }
    return ids;
  }, [selectedTodoId, visibleGraph.edges]);

  const [flowNodes, setFlowNodes] = useState<GraphFlowNode[]>([]);
  const [flowEdges, setFlowEdges] = useState<GraphFlowEdge[]>([]);

  useEffect(() => {
    let cancelled = false;

    async function runLayout() {
      const nodesById = new Map(visibleGraph.nodes.map((node) => [node.id, node]));
      const laidOutNodes: GraphFlowNode[] = [];
      const laidOutEdges: GraphFlowEdge[] = [];
      let cursorX = 24;
      let cursorY = 24;
      let tallestRow = 0;

      for (const component of visibleGraph.components) {
        const componentNodes = component.node_ids
          .map((nodeId) => nodesById.get(nodeId))
          .filter((node): node is TodoGraphNode => !!node);

        if (componentNodes.length === 0) continue;

        const { nodes, edges, width, height } = await layoutComponent(component, componentNodes, visibleGraph.edges);

        if (cursorX > 24 && cursorX + width > 2600) {
          cursorX = 24;
          cursorY += tallestRow + COMPONENT_GAP_Y;
          tallestRow = 0;
        }

        const rootLabel = component.root_summaries.length > 1 ? t('graph.rootsLabel') : t('graph.rootLabel');
        const statusLabelMap: Record<string, string> = {
          open: t('todo.open'),
          in_progress: t('todo.inProgress'),
          completed: t('todo.completed'),
        };

        for (const node of nodes) {
          laidOutNodes.push({
            ...node,
            position: { x: node.position.x + cursorX, y: node.position.y + cursorY },
            data: {
              todo: node.data.todo,
              selected: node.data.todo.id === selectedTodoId,
              neighbor: neighborIds.has(node.data.todo.id),
              statusLabel: statusLabelMap[node.data.todo.status],
              dueLabel: t('graph.dueLabel'),
              prereqLabel: t('graph.prerequisites'),
              dependentLabel: t('graph.dependents'),
              rootLabel,
              onSelect: setSelectedTodoId,
            },
          });
        }
        laidOutEdges.push(...edges);

        cursorX += width + COMPONENT_GAP_X;
        tallestRow = Math.max(tallestRow, height);
      }

      if (!cancelled) {
        setFlowNodes(laidOutNodes);
        setFlowEdges(laidOutEdges);
      }
    }

    void runLayout();

    return () => {
      cancelled = true;
    };
  }, [neighborIds, selectedTodoId, t, visibleGraph.components, visibleGraph.edges, visibleGraph.nodes]);

  useEffect(() => {
    if (!selectedTodoId || !flowRef.current) return;
    const node = flowNodes.find((item) => item.id === String(selectedTodoId));
    if (!node) return;
    flowRef.current.setCenter(node.position.x + NODE_WIDTH / 2, node.position.y + NODE_HEIGHT / 2, { duration: 400, zoom: 0.96 });
  }, [flowNodes, selectedTodoId]);

  const lockedPrerequisite = useMemo<TodoSummary | undefined>(() => {
    if (!prerequisiteForId || !data) return undefined;
    const matched = data.nodes.find((node) => node.id === prerequisiteForId);
    return matched ? { id: matched.id, code: matched.code, title: matched.title } : undefined;
  }, [data, prerequisiteForId]);

  const todoOptions = useMemo(() => {
    if (!data) return [];
    return data.nodes.map((node) => ({
      value: node.id,
      label: `${node.code} - ${node.title}`,
    }));
  }, [data]);

  const resetFilters = () => {
    setSearchParams({});
    setSelectedTodoId(null);
  };

  const updateParams = (updates: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams);
    for (const [key, value] of Object.entries(updates)) {
      if (value == null || value === '') next.delete(key);
      else next.set(key, value);
    }
    setSearchParams(next);
  };

  const handleNavigateTodo = (todoId: number) => {
    if (todoId === 0) {
      setSelectedTodoId(null);
      return;
    }

    if (visibleNodeIds.has(todoId)) {
      setSelectedTodoId(todoId);
      return;
    }

    updateParams({ focus: String(todoId) });
    setSelectedTodoId(todoId);
  };

  const handleRefresh = async () => {
    await refetch();
  };

  const hasVisibleGraph = flowNodes.length > 0;

  return (
    <div className="todo-graph-page">
      <div className="todo-graph-hero">
        <Space direction="vertical" size={4}>
          <Space>
            <Tag color="gold">{focusTodoId ? t('graph.focusBadge') : t('graph.allBadge')}</Tag>
            {focusComponent?.all_completed && <Tag color="green">{t('graph.componentCompleted')}</Tag>}
          </Space>
          <Title level={2} className="todo-graph-hero-title" style={{ margin: 0 }}>
            {t('graph.title')}
          </Title>
          <Paragraph className="todo-graph-hero-subtitle" style={{ margin: 0, maxWidth: 760 }}>
            {t('graph.subtitle')}
          </Paragraph>
        </Space>
      </div>

      <div className="todo-graph-toolbar">
        <div className="todo-graph-toolbar-group">
          <Segmented
            value={focusTodoId ? 'focus' : 'all'}
            options={[
              { label: t('graph.showAll'), value: 'all' },
              { label: t('graph.focusTodo'), value: 'focus' },
            ]}
            onChange={(value) => {
              if (value === 'all') updateParams({ focus: null });
            }}
          />
          <Select
            showSearch
            allowClear
            className="todo-graph-toolbar-search"
            placeholder={t('graph.focusPlaceholder')}
            options={todoOptions}
            value={focusTodoId || undefined}
            optionFilterProp="label"
            onChange={(value) => {
              updateParams({ focus: value ? String(value) : null });
              setSelectedTodoId(value || null);
            }}
          />
        </div>
        <div className="todo-graph-toolbar-group">
          <Space>
            <Text>{t('graph.showCompletedComponents')}</Text>
            <Switch checked={showCompletedComponents} onChange={(checked) => updateParams({ show_completed_components: checked ? '1' : null })} />
          </Space>
          <Space>
            <Text>{t('graph.hideCompletedNodes')}</Text>
            <Switch checked={hideCompletedNodes} onChange={(checked) => updateParams({ hide_completed_nodes: checked ? '1' : null })} />
          </Space>
          <Button icon={<ReloadOutlined />} onClick={handleRefresh}>{t('graph.refresh')}</Button>
          <Button onClick={resetFilters}>{t('graph.reset')}</Button>
        </div>
      </div>

      <div className="todo-graph-surface">
        <div className="todo-graph-backdrop" />
        {isLoading ? (
          <div className="todo-graph-empty"><Spin size="large" /></div>
        ) : !hasVisibleGraph ? (
          <div className="todo-graph-empty">
            <Card className="todo-graph-empty-card">
              <Empty
                image={<RadarChartOutlined style={{ fontSize: 48, color: '#335c67' }} />}
                description={
                  <Space direction="vertical" size={4}>
                    <Text strong>{t('graph.empty')}</Text>
                    <Text type="secondary">{t('graph.emptyHint')}</Text>
                  </Space>
                }
              />
            </Card>
          </div>
        ) : (
          <div className="todo-graph-canvas">
            <ReactFlow<GraphFlowNode, GraphFlowEdge>
              nodes={flowNodes}
              edges={flowEdges}
              nodeTypes={nodeTypes}
              fitView
              fitViewOptions={{ padding: 0.16 }}
              minZoom={0.2}
              maxZoom={1.5}
              nodesDraggable={false}
              nodesConnectable={false}
              elementsSelectable
              onNodeClick={(_, node) => setSelectedTodoId(Number(node.id))}
              onInit={(instance) => {
                flowRef.current = instance;
              }}
            >
              <Background gap={44} color="rgba(17, 44, 60, 0.08)" />
              <Controls showInteractive={false} />
              <MiniMap
                className="todo-graph-minimap"
                pannable
                zoomable
                nodeColor={(node) => {
                  const todo = (node as GraphFlowNode).data?.todo;
                  if (!todo) return 'rgba(17, 44, 60, 0.22)';
                  if (todo.status === 'completed') return '#2f855a';
                  if (todo.status === 'in_progress') return '#1d4ed8';
                  return '#94a3b8';
                }}
              />
            </ReactFlow>
          </div>
        )}
      </div>

      <Drawer
        title={t('graph.drawerTitle')}
        placement="right"
        width={520}
        open={!!selectedTodoId}
        onClose={() => setSelectedTodoId(null)}
        destroyOnClose={false}
      >
        <TodoDetailPanel
          todoId={selectedTodoId}
          onEdit={(id) => {
            setEditingId(id);
            setPrerequisiteForId(undefined);
            setFormOpen(true);
          }}
          onNavigate={handleNavigateTodo}
          onAddPrerequisite={(todoId) => {
            setEditingId(null);
            setPrerequisiteForId(todoId);
            setFormOpen(true);
          }}
          onDelete={() => {
            setSelectedTodoId(null);
            void refetch();
          }}
        />
      </Drawer>

      <TodoForm
        open={formOpen}
        todoId={editingId}
        onClose={() => {
          setFormOpen(false);
          setEditingId(null);
          setPrerequisiteForId(undefined);
          void refetch();
        }}
        lockedPrerequisite={lockedPrerequisite}
        onCreated={async (newTodoId) => {
          if (prerequisiteForId) {
            const currentTodo = await getTodo(prerequisiteForId);
            const existingIds = currentTodo.depends_on.map((dep) => dep.id);
            await updateTodo(prerequisiteForId, { depends_on_ids: [...existingIds, newTodoId] });
            updateParams({ focus: String(prerequisiteForId) });
            setSelectedTodoId(prerequisiteForId);
            await refetch();
          }
        }}
      />
    </div>
  );
}

export function TodoGraphPage() {
  return (
    <ReactFlowProvider>
      <TodoGraphPageInner />
    </ReactFlowProvider>
  );
}
