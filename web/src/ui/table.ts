// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { computed, defineComponent, h, ref, reactive, watch, nextTick } from 'vue';
import { ElTable, ElTableColumn, ElPagination, ElPopover, ElInput, ElButton, ElIcon } from 'element-plus';
import { Filter } from '@element-plus/icons-vue';

// HomeWaf column schema keeps server pagination and named business-cell slots
// stable while the rendering, accessibility and selection belong to Element Plus.
export default defineComponent({
  name: 'HwTable',
  inheritAttrs: false,
  props: [
    'data',
    'columns',
    'rowKey',
    'pagination',
    'loading',
    'selectedRowKeys',
    'expandedRow',
    'expandedRowKeys',
    'size',
    'maxHeight',
    'height',
    'bordered',
    'stripe',
    'displayColumns',
    'sort',
    'empty',
  ],
  emits: ['page-change', 'select-change', 'sort-change', 'filter-change', 'expand-change', 'row-click', 'change'],
  setup(props, { attrs, slots, emit, expose }) {
    const table = ref<any>();
    const rows = computed(() => (Array.isArray(props.data) ? props.data : []));
    const visibleColumns = computed(() => {
      const columns = props.columns || [];
      if (!Array.isArray(props.displayColumns)) return columns;
      const byKey = new Map(columns.map((column: any) => [column.colKey, column]));
      return [...new Set(props.displayColumns)].map((key) => byKey.get(key)).filter(Boolean);
    });
    const filterValues = reactive<Record<string, any>>({});
    const filterDrafts = reactive<Record<string, string>>({});
    const openFilter = ref('');
    const sortProp = () =>
      (props.columns || []).find((c: any) => (c.sortKey || c.colKey) === props.sort?.sortBy)?.colKey;
    let synchronizingSort = false;
    watch(
      () => props.sort,
      async () => {
        await nextTick();
        if (!table.value) return;
        synchronizingSort = true;
        const prop = sortProp();
        if (prop) table.value.sort(prop, props.sort.descending ? 'descending' : 'ascending');
        else table.value.clearSort();
        await nextTick();
        synchronizingSort = false;
      },
      { deep: true },
    );
    function publishFilter(key: string, value: any) {
      filterValues[key] = value;
      emit('filter-change', { ...filterValues });
      emit('change', { filter: { ...filterValues } }, { trigger: 'filter' });
      openFilter.value = '';
    }
    let synchronizing = false;
    const key = (row: any) => (typeof props.rowKey === 'function' ? props.rowKey(row) : row[props.rowKey || 'id']);
    watch(
      () => [props.data, props.selectedRowKeys],
      async () => {
        if (!Array.isArray(props.selectedRowKeys)) return;
        await nextTick();
        if (!table.value) return;
        synchronizing = true;
        table.value.clearSelection();
        for (const row of rows.value)
          if (props.selectedRowKeys.includes(key(row))) table.value.toggleRowSelection(row, true);
        await nextTick();
        synchronizing = false;
      },
      { deep: true },
    );
    expose({
      clearSelection: () => table.value?.clearSelection(),
      scrollTo: (...args: any[]) => table.value?.scrollTo(...args),
      refreshTable: () => table.value?.doLayout(),
      clearFilter: () => {
        Object.keys(filterValues).forEach((key) => delete filterValues[key]);
        Object.keys(filterDrafts).forEach((key) => delete filterDrafts[key]);
        openFilter.value = '';
        table.value?.clearFilter();
      },
    });
    function column(col: any, index: number): any {
      const selected = col.type === 'multiple' || col.type === 'single' || col.colKey === 'row-select';
      const filters = col.filter?.list?.map((x: any) => ({ text: x.label, value: x.value }));
      const options: any = {
        key: col.colKey || index,
        prop: col.colKey,
        label: typeof col.title === 'string' ? col.title : undefined,
        width: col.width,
        minWidth: col.minWidth ?? (col.width ? undefined : 120),
        align: col.align,
        fixed: col.fixed,
        showOverflowTooltip: col.ellipsis,
        sortable: col.sorter ? 'custom' : false,
        filters,
        filterMultiple: col.filter?.type !== 'single',
        columnKey: col.colKey,
        reserveSelection: selected,
      };
      if (selected) options.type = 'selection';
      if (col.colKey === 'serial-number') options.type = 'index';
      const cellSlots: any = {};
      if (col.children) cellSlots.default = () => col.children.map(column);
      else if (!selected && col.colKey !== 'serial-number')
        cellSlots.default = (scope: any) => {
          const context = { ...scope, rowIndex: scope.$index, col, colIndex: index };
          if (slots[col.colKey]) return slots[col.colKey]!(context);
          if (typeof col.cell === 'function') return col.cell(h, context);
          if (typeof col.cell === 'string' && slots[col.cell]) return slots[col.cell]!(context);
          const value = String(col.colKey || '')
            .split('.')
            .reduce((v: any, k: string) => v?.[k], scope.row);
          return value == null ? '' : String(value);
        };
      if (typeof col.title === 'function') cellSlots.header = () => col.title(h, { col, colIndex: index });
      if (col.filter?.type === 'input') {
        cellSlots.header = () =>
          h('span', { class: 'hw-column-header' }, [
            typeof col.title === 'function' ? col.title(h, { col, colIndex: index }) : col.title,
            h(
              ElPopover,
              {
                visible: openFilter.value === col.colKey,
                trigger: 'click',
                width: 260,
                'onUpdate:visible': (visible: boolean) => {
                  openFilter.value = visible ? col.colKey : '';
                },
              },
              {
                reference: () =>
                  h(
                    'button',
                    {
                      type: 'button',
                      class: ['hw-column-filter', { 'is-active': !!filterValues[col.colKey] }],
                      'aria-label': `筛选${col.title}`,
                      'aria-expanded': openFilter.value === col.colKey,
                      onClick: (event: Event) => event.stopPropagation(),
                    },
                    [h(ElIcon, {}, () => h(Filter))],
                  ),
                default: () =>
                  h('div', { class: 'hw-column-filter-panel' }, [
                    h(ElInput, {
                      modelValue: filterDrafts[col.colKey] || '',
                      'aria-label': `筛选${col.title}`,
                      placeholder: col.filter.props?.placeholder || '输入关键字',
                      clearable: true,
                      'onUpdate:modelValue': (value: string) => {
                        filterDrafts[col.colKey] = value;
                      },
                      onKeydown: (event: KeyboardEvent) => {
                        if (event.key === 'Enter') publishFilter(col.colKey, filterDrafts[col.colKey] || '');
                      },
                    }),
                    h('div', { class: 'hw-column-filter-actions' }, [
                      h(
                        ElButton,
                        {
                          onClick: () => {
                            filterDrafts[col.colKey] = '';
                            publishFilter(col.colKey, '');
                          },
                        },
                        () => '清除',
                      ),
                      h(
                        ElButton,
                        { type: 'primary', onClick: () => publishFilter(col.colKey, filterDrafts[col.colKey] || '') },
                        () => '应用',
                      ),
                    ]),
                  ]),
              },
            ),
          ]);
      }
      return h(ElTableColumn, options, cellSlots);
    }
    const page = (current: number, pageSize: number) => {
      const value = { current, pageSize, previous: props.pagination?.current || 1 };
      emit('page-change', value);
      emit('change', { pagination: value }, { trigger: 'pagination' });
    };
    return () =>
      h('div', { class: ['hw-table', 't-table', attrs.class] }, [
        h(
          ElTable,
          {
            ...attrs,
            ref: table,
            data: rows.value,
            rowKey: props.rowKey || 'id',
            size: props.size === 'medium' ? 'default' : props.size,
            maxHeight: props.maxHeight,
            height: props.height,
            border: props.bordered,
            stripe: props.stripe,
            defaultSort: sortProp()
              ? { prop: sortProp(), order: props.sort.descending ? 'descending' : 'ascending' }
              : undefined,
            expandRowKeys: props.expandedRowKeys,
            'aria-busy': !!props.loading,
            onSelectionChange: (rows: any[]) => {
              if (!synchronizing) {
                const keys = rows.map(key);
                emit('select-change', keys, { selectedRowData: rows, currentRowKey: keys.at(-1) });
              }
            },
            onSortChange: ({ prop, order }: any) => {
              if (synchronizingSort) return;
              const column = (props.columns || []).find((column: any) => column.colKey === prop);
              const sort = order ? { sortBy: column?.sortKey || prop, descending: order === 'descending' } : undefined;
              emit('sort-change', sort, { currentDataSource: props.data });
              emit('change', { sorter: sort }, { trigger: 'sorter' });
            },
            onFilterChange: (filters: any) => {
              Object.assign(filterValues, filters);
              emit('filter-change', { ...filterValues });
              emit('change', { filter: { ...filterValues } }, { trigger: 'filter' });
            },
            onExpandChange: (row: any, rows: any[]) =>
              emit('expand-change', rows.map(key), { expandedRowData: rows, currentRowData: row }),
            onRowClick: (row: any, col: any, e: Event) => emit('row-click', { row, col, e }),
          },
          {
            default: () => [
              props.expandedRow
                ? h(
                    ElTableColumn,
                    { type: 'expand' },
                    {
                      default: (scope: any) =>
                        props.expandedRow(h, { row: scope.row, index: scope.$index, rowIndex: scope.$index }),
                    },
                  )
                : null,
              ...visibleColumns.value.map(column),
            ],
            empty:
              slots.empty ||
              (() =>
                h('div', { class: 'table-empty-state' }, [
                  h('strong', props.loading ? '正在读取记录…' : props.empty || '暂无记录'),
                  props.loading ? null : h('p', '调整筛选条件后重试。'),
                ])),
          },
        ),
        props.pagination
          ? h(ElPagination, {
              class: 'hw-table-pagination',
              currentPage: props.pagination.current || 1,
              pageSize: props.pagination.pageSize || 20,
              total: props.pagination.total || 0,
              pageSizes: props.pagination.pageSizeOptions || [10, 20, 50, 100],
              layout: 'total, sizes, prev, pager, next',
              pagerCount: 5,
              'onUpdate:currentPage': (current: number) => page(current, props.pagination.pageSize || 20),
              'onUpdate:pageSize': (size: number) => page(1, size),
            })
          : null,
        props.loading ? h('div', { class: 'hw-table-loading', role: 'status' }, '正在读取记录…') : null,
      ]);
  },
});
