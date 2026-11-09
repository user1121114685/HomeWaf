// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { useRouter } from 'vue-router';
import { defineComponent, h, ref, createVNode, render, reactive, Fragment, computed } from 'vue';
import * as E from 'element-plus';
import HwTable from './table';
import HwUpload from './upload';
import HwIcon, { icons } from './icons';
import zhCN from 'element-plus/es/locale/lang/zh-cn';
import enUS from 'element-plus/es/locale/lang/en';

export type TableSort = { sortBy: string; descending: boolean } | undefined;
export type TableChangeContext = any;
export type TableChangeData = any;
export type PopupVisibleChangeContext = any;
const renderNative = (component: any, props: any, slots?: any) => h(component, props, slots);
const size = (value: any) => (value === 'medium' ? 'default' : value);
const theme = (value: any) => (value === 'default' ? 'info' : value === 'error' ? 'danger' : value);
const popupPlacement = (value: any) =>
  (
    ({
      'top-left': 'top-start',
      'top-right': 'top-end',
      'bottom-left': 'bottom-start',
      'bottom-right': 'bottom-end',
      'left-top': 'left-start',
      'left-bottom': 'left-end',
      'right-top': 'right-start',
      'right-bottom': 'right-end',
    }) as Record<string, string>
  )[value] ||
  value ||
  'bottom';
function attributes(raw: any) {
  return new Proxy(raw, {
    get(target, key, receiver) {
      const direct = Reflect.get(target, key, receiver);
      return direct !== undefined || typeof key !== 'string'
        ? direct
        : Reflect.get(
            target,
            key.replace(/[A-Z]/g, (c) => '-' + c.toLowerCase()),
            receiver,
          );
    },
  });
}
let applicationContext: any = null;
const modelProps = { modelValue: null, value: null, disabled: Boolean, size: String };
function input(name: string, component: any, extra: (p: any, a: any) => any = () => ({})) {
  return defineComponent({
    name,
    inheritAttrs: false,
    props: modelProps,
    emits: ['update:modelValue', 'update:value', 'input', 'change', 'enter', 'blur', 'focus', 'clear'],
    setup(p, { attrs: a, slots, emit, expose }) {
      a = attributes(a);
      const el = ref<any>();
      expose({
        focus: () => el.value?.focus(),
        blur: () => el.value?.blur(),
        clear: () => {
          emit('update:modelValue', '');
          emit('change', '');
        },
      });
      return () =>
        h(
          component,
          {
            ...a,
            ...extra(p, a),
            ref: el,
            modelValue: p.modelValue !== undefined ? p.modelValue : p.value,
            disabled: p.disabled,
            size: size(p.size),
            'onUpdate:modelValue': (v: any) => {
              emit('update:modelValue', v);
              emit('update:value', v);
              emit('input', v);
              emit('change', v, { trigger: 'input' });
            },
            onKeyup: (e: KeyboardEvent) => {
              if (e.key === 'Enter') emit('enter', p.modelValue ?? p.value, { e });
            },
            onBlur: (e: Event) => emit('blur', p.modelValue ?? p.value, { e }),
            onFocus: (e: Event) => emit('focus', p.modelValue ?? p.value, { e }),
            onClear: () => emit('clear'),
          },
          { ...slots, prefix: slots.prefix || slots['prefix-icon'], suffix: slots.suffix || slots['suffix-icon'] },
        );
    },
  });
}
function plain(name: string, component: any, transform: (a: any) => any = () => ({})) {
  return defineComponent({
    name,
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      attrs = attributes(attrs);
      return () => {
        const extra = transform(attrs);
        return h(
          component,
          { ...attrs, ...extra, class: [attrs.class, extra.class], style: [attrs.style, extra.style] },
          slots,
        );
      };
    },
  });
}
const HwButton = plain('HwButton', E.ElButton, (a) => ({
  class: ['hw-button', 't-button', { 'hw-button-square': a.shape === 'square' }],
  type: theme(a.theme),
  plain: a.variant === 'outline',
  text: a.variant === 'text',
  link: a.variant === 'text',
  nativeType: a.type || 'button',
  size: size(a.size),
  circle: a.shape === 'circle',
  style: [a.style, a.block !== undefined && a.block !== false ? { width: '100%' } : null],
}));
const HwTag = plain('HwTag', E.ElTag, (a) => ({
  type: theme(a.theme),
  effect: a.variant === 'outline' ? 'plain' : a.variant === 'dark' ? 'dark' : 'light',
  size: size(a.size),
}));
const HwFormItem = defineComponent({
  name: 'HwFormItem',
  inheritAttrs: false,
  setup(_, { attrs: raw, slots }) {
    const attrs = attributes(raw);
    return () =>
      h(
        E.ElFormItem,
        { ...attrs, class: ['hw-form-item', attrs.class], prop: attrs.name, label: attrs.label, error: attrs.error },
        {
          ...slots,
          default: () => [
            slots.default?.(),
            slots.help || attrs.help
              ? h('div', { class: 'hw-form-help t-input__help' }, slots.help?.() || attrs.help)
              : null,
          ],
        },
      );
  },
});
const HwForm = defineComponent({
  name: 'HwForm',
  inheritAttrs: false,
  props: ['data', 'rules'],
  emits: ['submit', 'reset'],
  setup(p, { attrs, slots, emit, expose }) {
    attrs = attributes(attrs);
    const form = ref<any>();
    const rules = computed(() =>
      Object.fromEntries(
        Object.entries(p.rules || {}).map(([key, items]: any) => [
          key,
          (Array.isArray(items) ? items : [items]).map((rule: any) => {
            if (['error', 'warning', 'success'].includes(rule.type)) {
              rule = { ...rule };
              delete rule.type;
            }
            if (!rule.validator) return rule;
            const validate = rule.validator;
            return {
              ...rule,
              async validator(_rule: any, value: any) {
                const result = await validate(value);
                if (result === false || result?.result === false)
                  throw new Error(result?.message || rule.message || 'Invalid value');
              },
            };
          }),
        ]),
      ),
    );
    async function validate() {
      try {
        await form.value.validate();
        return true;
      } catch (errors) {
        return errors;
      }
    }
    expose({
      validate,
      validateField: (...args: any[]) => form.value?.validateField(...args),
      clearValidate: (...args: any[]) => form.value?.clearValidate(...args),
      reset: () => form.value?.resetFields(),
      resetFields: () => form.value?.resetFields(),
    });
    return () =>
      renderNative(
        E.ElForm,
        {
          ...attrs,
          ref: form,
          model: p.data,
          rules: rules.value,
          inline: attrs.layout === 'inline',
          class: ['hw-form', attrs.class],
          labelWidth: attrs.labelWidth ?? (attrs.layout === 'inline' ? 'auto' : 100),
          labelPosition: attrs.layout === 'vertical' ? 'top' : (attrs.labelAlign ?? 'right'),
          onSubmit: async (e: Event) => {
            e.preventDefault();
            const result = await validate();
            emit('submit', {
              e,
              result,
              validateResult: result,
              firstError:
                result === true
                  ? ''
                  : Object.values(result as Record<string, { message?: string }[]>).flat()[0]?.message,
            });
          },
          onReset: () => {
            form.value?.resetFields();
            emit('reset');
          },
        },
        slots,
      );
  },
});
function dialog(drawer = false) {
  return defineComponent({
    name: drawer ? 'HwDrawer' : 'HwDialog',
    inheritAttrs: false,
    props: [
      'visible',
      'modelValue',
      'header',
      'confirmBtn',
      'cancelBtn',
      'footer',
      'width',
      'placement',
      'closeOnOverlayClick',
      'destroyOnClose',
    ],
    emits: ['update:visible', 'update:modelValue', 'confirm', 'cancel', 'close', 'closed', 'opened'],
    setup(p, { attrs, slots, emit, expose }) {
      attrs = attributes(attrs);
      const close = (trigger: string) => {
        emit('update:visible', false);
        emit('update:modelValue', false);
        emit('close', { trigger });
      };
      expose({ hide: () => close('programmatic') });
      const action = (value: any, fallback: string, kind: string) =>
        value === null
          ? null
          : renderNative(
              E.ElButton,
              {
                ...(typeof value === 'object' ? value : {}),
                type: kind === 'confirm' ? 'primary' : undefined,
                onClick: (e: Event) => {
                  emit(kind as any, { e });
                  if (kind === 'cancel') close('cancel');
                },
              },
              () => (typeof value === 'string' ? value : value?.content || fallback),
            );
      return () =>
        renderNative(
          drawer ? E.ElDrawer : E.ElDialog,
          {
            ...attrs,
            modelValue: p.visible ?? p.modelValue ?? false,
            title: typeof p.header === 'string' ? p.header : undefined,
            width: p.width,
            size: p.width ?? attrs.size,
            direction: ({ left: 'ltr', right: 'rtl', top: 'ttb', bottom: 'btt' } as any)[p.placement] || 'rtl',
            destroyOnClose: p.destroyOnClose,
            closeOnClickModal: p.closeOnOverlayClick ?? false,
            'onUpdate:modelValue': (v: boolean) => {
              emit('update:visible', v);
              emit('update:modelValue', v);
            },
            onClose: () => emit('close', { trigger: 'close' }),
            onClosed: () => emit('closed'),
            onOpened: () => emit('opened'),
          },
          {
            default: slots.body || slots.default,
            header: slots.header,
            footer:
              p.footer === false
                ? undefined
                : slots.footer ||
                  (() => [action(p.cancelBtn, '取消', 'cancel'), action(p.confirmBtn, '确定', 'confirm')]),
          },
        );
    },
  });
}
const HwSelect = defineComponent({
  name: 'HwSelect',
  inheritAttrs: false,
  props: ['modelValue', 'value', 'options'],
  emits: ['update:modelValue', 'change', 'clear'],
  setup(p, { attrs, slots, emit, expose }) {
    attrs = attributes(attrs);
    const el = ref<any>();
    expose({ focus: () => el.value?.focus(), blur: () => el.value?.blur() });
    return () =>
      renderNative(
        E.ElSelect,
        {
          ...attrs,
          ref: el,
          modelValue: p.modelValue ?? p.value,
          size: size(attrs.size),
          filterable: attrs.filterable ?? attrs.filter,
          'onUpdate:modelValue': (v: any) => {
            emit('update:modelValue', v);
            emit('change', v);
          },
          onClear: () => emit('clear'),
        },
        {
          ...slots,
          default: () =>
            p.options
              ? p.options.map((x: any) =>
                  renderNative(E.ElOption, { ...x, key: x.value, label: x.label, value: x.value }),
                )
              : slots.default?.(),
        },
      );
  },
});
const HwDropdown = defineComponent({
  name: 'HwDropdown',
  inheritAttrs: false,
  props: ['options', 'trigger'],
  emits: ['click'],
  setup(p, { attrs, slots, emit }) {
    return () =>
      renderNative(
        E.ElDropdown,
        {
          ...attrs,
          trigger: p.trigger || 'hover',
          onCommand: (value: any) => {
            const item = (p.options || []).find((x: any) => x.value === value);
            emit('click', item || value);
          },
        },
        {
          default: slots.default,
          dropdown:
            slots.dropdown ||
            (() =>
              renderNative(E.ElDropdownMenu, {}, () =>
                p.options
                  ? (p.options || []).map((x: any) =>
                      renderNative(
                        E.ElDropdownItem,
                        { command: x.value, disabled: x.disabled, divided: x.divider, onClick: () => x.onClick?.(x) },
                        () => [x.prefixIcon?.(h), x.content ?? x.label],
                      ),
                    )
                  : slots['dropdown-menu']?.(),
              )),
        },
      );
  },
});
const HwDescriptions = plain('HwDescriptions', E.ElDescriptions, (a) => ({
  column: a.column || 3,
  border: a.bordered,
  labelWidth: a.labelWidth,
}));
const HwTabs = input('HwTabs', E.ElTabs, (_p, a) => ({
  type: a.theme === 'card' ? 'card' : undefined,
  tabPosition: a.placement || 'top',
  onTabClick: (pane: any) => a.onChange?.(pane.props.name),
}));
const HwLoading = defineComponent({
  name: 'HwLoading',
  props: ['loading', 'text'],
  setup(p, { slots }) {
    return () =>
      h('div', { class: 'hw-loading', 'aria-busy': !!p.loading }, [
        slots.default?.(),
        p.loading ? h('div', { class: 'hw-loading-mask', role: 'status' }, p.text || 'Loading…') : null,
      ]);
  },
});
const HwPopup = defineComponent({
  name: 'HwPopup',
  inheritAttrs: false,
  props: ['visible', 'content', 'trigger', 'placement'],
  emits: ['update:visible', 'visible-change'],
  setup(p, { attrs, slots, emit }) {
    attrs = attributes(attrs);
    return () =>
      renderNative(
        E.ElPopover,
        {
          ...attrs,
          visible: p.visible,
          content: typeof p.content === 'string' ? p.content : undefined,
          width: attrs.width ?? 'auto',
          popperClass: ['hw-popup', attrs.overlayClassName, attrs.popperClass].filter(Boolean).join(' '),
          popperStyle: [attrs.overlayStyle, attrs.overlayInnerStyle, attrs.popperStyle],
          trigger: p.trigger || 'hover',
          placement: popupPlacement(p.placement),
          'onUpdate:visible': (v: boolean) => {
            emit('update:visible', v);
            emit('visible-change', v);
          },
        },
        { reference: slots.default, default: slots.content },
      );
  },
});
const HwMenuItem = defineComponent({
  name: 'HwMenuItem',
  inheritAttrs: false,
  props: ['value', 'to', 'href'],
  setup(p, { attrs, slots }) {
    const router = useRouter();
    return () =>
      renderNative(
        E.ElMenuItem,
        {
          ...attrs,
          index: String(p.value ?? p.to ?? ''),
          onClick: (event: any) => {
            (attrs.onClick as any)?.(event);
            if (p.href) {
              const url = new URL(p.href, location.href);
              if (['https:', 'http:'].includes(url.protocol)) window.open(url.href, '_blank', 'noopener,noreferrer');
            } else if (p.to) {
              router.push(p.to);
            }
          },
        },
        { default: () => [slots.icon?.(), slots.default?.()] },
      );
  },
});
const HwSubmenu = defineComponent({
  name: 'HwSubmenu',
  inheritAttrs: false,
  props: ['value', 'title'],
  setup(p, { attrs, slots }) {
    return () =>
      renderNative(
        E.ElSubMenu,
        { ...attrs, index: String(p.value ?? '') },
        { default: slots.default, title: () => [slots.icon?.(), slots.title?.() || p.title] },
      );
  },
});
const HwMenu = defineComponent({
  name: 'HwMenu',
  inheritAttrs: false,
  setup(_, { attrs, slots }) {
    const a = attributes(attrs);
    return () => {
      const horizontal = a.mode === 'horizontal';
      return h(
        'div',
        {
          class: [a.class, 'hw-menu', horizontal ? 'hw-head-menu' : 'hw-side-menu'],
          style: [a.style, horizontal ? {} : { width: a.collapsed ? '72px' : a.width || '232px' }],
        },
        [
          slots.logo ? h('div', { class: 'hw-menu-logo' }, slots.logo()) : null,
          h(
            E.ElMenu,
            {
              ...a,
              class: 'hw-menu-body',
              style: undefined,
              defaultActive: a.value,
              defaultOpeneds: a.defaultExpanded || [],
              collapse: !horizontal && a.collapsed,
              mode: horizontal ? 'horizontal' : 'vertical',
              uniqueOpened: true,
              onSelect: (value: string) => a.onChange?.(value),
            },
            slots.default ? { default: slots.default } : undefined,
          ),
          slots.operations ? h('div', { class: 'hw-menu-operations' }, slots.operations()) : null,
        ],
      );
    };
  },
});
const HwConfig = defineComponent({
  name: 'HwConfigProvider',
  props: ['locale'],
  setup(p, { slots }) {
    return () =>
      renderNative(
        E.ElConfigProvider,
        { locale: String(p.locale || localStorage.getItem('lang') || 'zh_CN').startsWith('en') ? enUS : zhCN },
        slots,
      );
  },
});
const HwIconComponent = HwIcon;
const HwCard = defineComponent({
  name: 'HwCard',
  inheritAttrs: false,
  setup(_, { attrs: raw, slots }) {
    const attrs = attributes(raw);
    return () =>
      h(
        E.ElCard,
        {
          ...attrs,
          class: ['hw-card', 't-card', attrs.class],
          shadow: attrs.bordered === false ? 'never' : attrs.shadow,
        },
        {
          ...slots,
          header:
            slots.header ||
            (slots.title || attrs.title || attrs.subtitle || slots.actions
              ? () =>
                  h('div', { class: 'hw-card-heading' }, [
                    h('div', { class: 'hw-card-titles' }, [
                      h('div', { class: 'hw-card-title t-card__title' }, slots.title?.() || attrs.title),
                      slots.subtitle || attrs.subtitle
                        ? h('div', { class: 'hw-card-subtitle' }, slots.subtitle?.() || attrs.subtitle)
                        : null,
                    ]),
                    slots.actions ? h('div', { class: 'hw-card-actions' }, slots.actions()) : null,
                  ])
              : undefined),
        },
      );
  },
});
const gridBreakpoints = [
  ['xs', 0],
  ['sm', 768],
  ['md', 992],
  ['lg', 1200],
  ['xl', 1400],
  ['xxl', 1880],
] as const;
const viewportWidth = ref(window.innerWidth);
const resizeGrid = () => {
  viewportWidth.value = window.innerWidth;
};
window.addEventListener('resize', resizeGrid, { passive: true });
if (import.meta.hot) import.meta.hot.dispose(() => window.removeEventListener('resize', resizeGrid));
const HwRow = defineComponent({
  name: 'HwRow',
  inheritAttrs: false,
  setup(_, { attrs, slots }) {
    const gap = (value: any) => {
      if (!value || typeof value !== 'object') return Number(value) || 0;
      let result = 0;
      for (const [breakpoint, minimum] of gridBreakpoints)
        if (viewportWidth.value >= minimum && value[breakpoint] != null) result = Number(value[breakpoint]) || 0;
      return result;
    };
    return () => {
      const gutters = Array.isArray(attrs.gutter) ? attrs.gutter : [attrs.gutter, 0];
      return h(
        E.ElRow,
        {
          ...attrs,
          class: ['hw-row', 't-row', attrs.class],
          gutter: gap(gutters[0]),
          align: attrs.align as 'top' | 'middle' | 'bottom' | undefined,
          style: [{ rowGap: `${gap(gutters[1])}px` }, attrs.style],
        },
        slots,
      );
    };
  },
});
const columnSize = (value: any) => {
  if (value == null) return undefined;
  if (typeof value !== 'object') return Number(value) * 2;
  return Object.fromEntries(
    Object.entries(value).map(([key, number]) => [
      key,
      ['span', 'offset', 'push', 'pull'].includes(key) ? Number(number) * 2 : number,
    ]),
  );
};
const HwCol = plain('HwCol', E.ElCol, (a) => {
  let current = { span: a.span ?? 12, offset: a.offset || 0, push: a.push || 0, pull: a.pull || 0 };
  for (const [breakpoint, minimum] of gridBreakpoints) {
    if (viewportWidth.value >= minimum && a[breakpoint] != null)
      current = { ...current, ...(typeof a[breakpoint] === 'object' ? a[breakpoint] : { span: a[breakpoint] }) };
  }
  return {
    ...(columnSize(current) as Record<string, number>),
    xs: undefined,
    sm: undefined,
    md: undefined,
    lg: undefined,
    xl: undefined,
    xxl: undefined,
    style:
      a.flex != null
        ? {
            flex:
              typeof a.flex === 'number'
                ? `${a.flex} 1 0%`
                : /^\d+(px|%|rem|em)$/.test(a.flex)
                  ? `0 0 ${a.flex}`
                  : a.flex,
            maxWidth: 'none',
            width: 'auto',
            minWidth: 0,
          }
        : undefined,
  };
});

const components: Record<string, any> = {
  'hw-button': HwButton,
  'hw-tag': HwTag,
  'hw-form': HwForm,
  'hw-form-item': HwFormItem,
  'hw-input': input('HwInput', E.ElInput, (_p, a) => ({
    showPassword: a.type === 'password',
    maxlength: a.maxlength ?? a.maxcharacter,
    showWordLimit: a.showLimitNumber,
  })),
  'hw-textarea': input('HwTextarea', E.ElInput, (_p, a) => ({
    type: 'textarea',
    autosize: a.autosize,
    rows: a.rows || 3,
    showWordLimit: a.showLimitNumber,
  })),
  'hw-input-number': input('HwInputNumber', E.ElInputNumber),
  'hw-switch': input('HwSwitch', E.ElSwitch, (_p, a) => ({
    activeValue: a.customValue?.[0] ?? true,
    inactiveValue: a.customValue?.[1] ?? false,
  })),
  'hw-select': HwSelect,
  'hw-option': E.ElOption,
  'hw-option-group': E.ElOptionGroup,
  'hw-radio-group': input('HwRadioGroup', E.ElRadioGroup),
  'hw-radio': E.ElRadio,
  'hw-radio-button': E.ElRadioButton,
  'hw-checkbox': input('HwCheckbox', E.ElCheckbox, (p) => ({ value: p.value })),
  'hw-checkbox-group': input('HwCheckboxGroup', E.ElCheckboxGroup),
  'hw-dialog': dialog(),
  'hw-drawer': dialog(true),
  'hw-table': HwTable,
  'hw-card': HwCard,
  'hw-row': HwRow,
  'hw-col': HwCol,
  'hw-space': plain('HwSpace', E.ElSpace, (a) => ({
    wrap: a.breakLine !== false,
    direction: a.direction || 'horizontal',
    alignment: a.align,
  })),
  'hw-tooltip': plain('HwTooltip', E.ElTooltip, (a) => ({ placement: popupPlacement(a.placement) })),
  'hw-alert': defineComponent({
    name: 'HwAlert',
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () =>
        h(
          E.ElAlert,
          {
            ...attrs,
            title: String(attrs.title ?? attrs.message ?? ''),
            type: (attrs.theme === 'danger' ? 'error' : String(attrs.theme || 'info')) as
              'error' | 'info' | 'primary' | 'success' | 'warning',
            description: attrs.description == null ? undefined : String(attrs.description),
            showIcon: attrs.showIcon !== false,
          },
          { ...slots, default: slots.message || slots.default },
        );
    },
  }),
  'hw-link': plain('HwLink', E.ElLink, (a) => ({ type: theme(a.theme) })),
  'hw-divider': E.ElDivider,
  'hw-icon': HwIconComponent,
  'hw-tabs': HwTabs,
  'hw-tab-panel': plain('HwTabPanel', E.ElTabPane, (a) => ({ name: a.value, label: a.label, lazy: a.lazy })),
  'hw-dropdown': HwDropdown,
  'hw-dropdown-menu': E.ElDropdownMenu,
  'hw-dropdown-item': plain('HwDropdownItem', E.ElDropdownItem, (a) => ({ command: a.value })),
  'hw-date-picker': input('HwDatePicker', E.ElDatePicker, (_p, a) => ({
    type: a.mode === 'date' ? 'date' : a.mode || 'date',
    valueFormat: 'YYYY-MM-DD',
    format: a.format,
  })),
  'hw-date-range-picker': input('HwDateRangePicker', E.ElDatePicker, (_p, a) => ({
    type: a.enableTimePicker ? 'datetimerange' : 'daterange',
    valueFormat: a.enableTimePicker ? 'YYYY-MM-DD HH:mm:ss' : 'YYYY-MM-DD',
    startPlaceholder: a.placeholder?.[0],
    endPlaceholder: a.placeholder?.[1],
  })),
  'hw-descriptions': HwDescriptions,
  'hw-descriptions-item': E.ElDescriptionsItem,
  'hw-loading': HwLoading,
  'hw-popup': HwPopup,
  'hw-progress': plain('HwProgress', E.ElProgress, (a) => ({
    percentage: a.percentage ?? a.value ?? 0,
    status: a.status === 'success' ? 'success' : a.status === 'error' ? 'exception' : undefined,
    type: a.theme === 'circle' ? 'circle' : 'line',
  })),
  'hw-collapse': input('HwCollapse', E.ElCollapse),
  'hw-collapse-panel': plain('HwCollapsePanel', E.ElCollapseItem, (a) => ({
    name: a.value,
    title: a.header,
    disabled: a.disabled,
  })),
  'hw-steps': plain('HwSteps', E.ElSteps, (a) => ({ active: a.current, processStatus: a.status })),
  'hw-step-item': plain('HwStepItem', E.ElStep, (a) => ({ title: a.title, description: a.content, status: a.status })),
  'hw-avatar': E.ElAvatar,
  'hw-avatar-group': plain('HwAvatarGroup', 'div', () => ({ class: 'hw-avatar-group' })),
  'hw-layout': E.ElContainer,
  'hw-content': E.ElMain,
  'hw-aside': plain('HwAside', E.ElAside, (a) => ({ width: a.width || 'auto' })),
  'hw-header': E.ElHeader,
  'hw-footer': E.ElFooter,
  'hw-menu': HwMenu,
  'hw-head-menu': plain('HwHeadMenu', HwMenu, () => ({ mode: 'horizontal' })),
  'hw-menu-item': HwMenuItem,
  'hw-submenu': HwSubmenu,
  'hw-breadcrumb': E.ElBreadcrumb,
  'hw-breadcrumbItem': E.ElBreadcrumbItem,
  'hw-breadcrumb-item': E.ElBreadcrumbItem,
  'hw-badge': plain('HwBadge', E.ElBadge, (a) => ({
    value: a.count,
    isDot: a.dot,
    hidden: a.count === 0 && !a.showZero,
  })),
  'hw-empty': plain('HwEmpty', E.ElEmpty, (a) => ({ description: a.description || a.title })),
  'hw-config-provider': HwConfig,
  'hw-list': plain('HwList', 'ul', () => ({ class: 'hw-list' })),
  'hw-list-item': plain('HwListItem', 'li', () => ({ class: 'hw-list-item' })),
  'hw-list-item-meta': defineComponent({
    name: 'HwListItemMeta',
    props: ['title', 'description'],
    setup(p, { slots }) {
      return () =>
        h('div', { class: 'hw-list-item-meta' }, [
          slots.avatar?.(),
          h('div', [slots.title?.() || p.title, slots.description?.() || h('p', p.description)]),
        ]);
    },
  }),
  'hw-tree': plain('HwTree', E.ElTree, (a) => ({
    nodeKey: a.keys?.value || 'value',
    props: { label: a.keys?.label || 'label', children: a.keys?.children || 'children' },
    showCheckbox: a.checkable,
    defaultCheckedKeys: a.value,
    defaultExpandedKeys: a.expanded,
  })),
  'hw-pagination': plain('HwPagination', E.ElPagination, (a) => ({
    currentPage: a.current,
    pageSize: a.pageSize,
    total: a.total,
    layout: 'total, sizes, prev, pager, next, jumper',
    'onUpdate:currentPage': (current: number) => a.onChange?.({ current, pageSize: a.pageSize }),
  })),
  'hw-color-picker-panel': input('HwColorPickerPanel', E.ElColorPicker),
  'hw-swiper': E.ElCarousel,
  'hw-swiper-item': E.ElCarouselItem,
  'hw-sticky-tool': plain('HwStickyTool', 'aside', () => ({ class: 'hw-sticky-tool' })),
  'hw-sticky-item': HwButton,
  'hw-tag-input': input('HwTagInput', E.ElSelect, () => ({
    multiple: true,
    filterable: true,
    allowCreate: true,
    defaultFirstOption: true,
  })),
  'hw-upload': HwUpload,
  'hw-popconfirm': plain('HwPopconfirm', E.ElPopconfirm, (a) => ({
    title: a.content,
    confirmButtonText: a.confirmBtn,
    cancelButtonText: a.cancelBtn,
    onConfirm: a.onConfirm,
  })),
};

function notify(kind: string, options: any, duration?: number) {
  const input = typeof options === 'string' ? { content: options } : options || {};
  const instance = E.ElMessage({
    message: input.content,
    type: kind === 'loading' ? 'info' : (kind as any),
    duration: kind === 'loading' ? 0 : duration !== undefined ? duration : (input.duration ?? 3000),
    grouping: true,
  });
  const result: any = Promise.resolve(instance);
  result.close = () => instance.close();
  return result;
}
export const MessagePlugin: any = Object.assign(
  (options: any) => notify(options?.theme || 'info', options),
  Object.fromEntries(
    ['success', 'error', 'warning', 'info', 'loading'].map((kind) => [
      kind,
      (options: any, duration?: number) => notify(kind, options, duration),
    ]),
  ),
);
MessagePlugin.closeAll = () => E.ElMessage.closeAll();
export const NotifyPlugin: any = Object.assign(
  (options: any) =>
    E.ElNotification({ ...options, title: options.title || '', message: options.content || options.message }),
  Object.fromEntries(
    ['success', 'error', 'warning', 'info'].map((kind) => [
      kind,
      (options: any) =>
        E.ElNotification({ type: kind as any, ...options, message: options.content || options.message }),
    ]),
  ),
);
function openDialog(options: any) {
  const state = reactive({ ...options, visible: true });
  const mount = document.createElement('div');
  document.body.append(mount);
  let rootNode: any;
  const controller = {
    hide: () => {
      state.visible = false;
    },
    destroy: () => {
      render(null, mount);
      mount.remove();
    },
    update: (value: any) => Object.assign(state, value),
    show: () => {
      state.visible = true;
    },
  };
  rootNode = createVNode(
    defineComponent({
      render: () =>
        h(
          components['hw-dialog'],
          {
            ...state,
            'onUpdate:visible': (v: boolean) => (state.visible = v),
            onConfirm: (ctx: any) =>
              state.onConfirm ? state.onConfirm({ ...ctx, dialog: controller }) : controller.hide(),
            onCancel: (ctx: any) => {
              state.onCancel?.({ ...ctx, dialog: controller });
              controller.hide();
            },
            onClose: (ctx: any) => state.onClose?.({ ...ctx, dialog: controller }),
            onClosed: () => controller.destroy(),
          },
          {
            default: () =>
              typeof state.body === 'function'
                ? state.body(h)
                : typeof state.content === 'function'
                  ? state.content(h)
                  : state.body || state.content,
          },
        ),
    }),
  );
  rootNode.appContext = applicationContext;
  render(rootNode, mount);
  return controller;
}
export const DialogPlugin: any = Object.assign(openDialog, {
  confirm: openDialog,
  alert: (options: any) => openDialog({ ...options, cancelBtn: null }),
});
export default {
  install(app: any) {
    applicationContext = app._context;
    for (const [name, component] of Object.entries(components)) app.component(name, component);
    for (const [name, icon] of Object.entries(icons)) app.component(name, icon);
    Object.assign(app.config.globalProperties, {
      $message: MessagePlugin,
      $notify: NotifyPlugin,
      $dialog: DialogPlugin,
    });
  },
};
