// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { defineComponent, h, ref, watch, nextTick } from 'vue';
import { ElTable, ElTableColumn, ElPagination } from 'element-plus';

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
  ],
  emits: ['page-change', 'select-change', 'sort-change', 'filter-change', 'expand-change', 'row-click', 'change'],
  setup(props, { attrs, slots, emit, expose }) {
    const table = ref<any>();
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
        for (const row of props.data || [])
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
    });
    function column(col: any, index: number): any {
      const selected = col.type === 'multiple' || col.type === 'single' || col.colKey === 'row-select';
      const filters = col.filter?.list?.map((x: any) => ({ text: x.label, value: x.value }));
      const options: any = {
        key: col.colKey || index,
        prop: col.colKey,
        label: typeof col.title === 'string' ? col.title : undefined,
        width: col.width,
        minWidth: col.minWidth,
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
            data: props.data || [],
            rowKey: props.rowKey || 'id',
            size: props.size === 'medium' ? 'default' : props.size,
            maxHeight: props.maxHeight,
            height: props.height,
            border: props.bordered,
            stripe: props.stripe,
            expandRowKeys: props.expandedRowKeys,
            'aria-busy': !!props.loading,
            onSelectionChange: (rows: any[]) => {
              if (!synchronizing) {
                const keys = rows.map(key);
                emit('select-change', keys, { selectedRowData: rows, currentRowKey: keys.at(-1) });
              }
            },
            onSortChange: ({ prop, order }: any) => {
              const sort = order ? { sortBy: prop, descending: order === 'descending' } : undefined;
              emit('sort-change', sort, { currentDataSource: props.data });
              emit('change', { sorter: sort }, { trigger: 'sorter' });
            },
            onFilterChange: (filters: any) => {
              emit('filter-change', filters);
              emit('change', { filter: filters }, { trigger: 'filter' });
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
              ...(props.columns || []).map(column),
            ],
            empty: slots.empty,
          },
        ),
        props.pagination
          ? h(ElPagination, {
              class: 'hw-table-pagination',
              currentPage: props.pagination.current || 1,
              pageSize: props.pagination.pageSize || 20,
              total: props.pagination.total || 0,
              pageSizes: props.pagination.pageSizeOptions || [10, 20, 50, 100],
              layout: 'total, sizes, prev, pager, next, jumper',
              'onUpdate:currentPage': (current: number) => page(current, props.pagination.pageSize || 20),
              'onUpdate:pageSize': (size: number) => page(1, size),
            })
          : null,
        props.loading ? h('div', { class: 'hw-table-loading', role: 'status' }, 'Loading…') : null,
      ]);
  },
});
