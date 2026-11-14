<template>
  <div
    class="console-shell"
    :class="{ 'is-compact': setting.isSidebarCompact }"
    @keydown.esc="closeMobileNavigation"
    @keydown.tab="trapMobileFocus"
  >
    <router-view v-if="$route.meta.fullscreen" />
    <template v-else>
      <a class="workspace-skip" href="#console-main" @click.prevent="$el.querySelector('#console-main')?.focus()"
        >跳到主要内容</a
      >
      <button
        v-if="mobile && !setting.isSidebarCompact"
        class="mobile-nav-backdrop"
        tabindex="-1"
        aria-label="关闭导航菜单"
        @click="closeMobileNavigation"
      ></button>
      <div class="console-layout">
        <aside class="console-sidebar" aria-label="功能导航">
          <side-nav :menu="menuRouters" :collapsed="setting.isSidebarCompact && !mobile" />
        </aside>
        <div class="console-body" :inert="mobile && !setting.isSidebarCompact">
          <div class="console-topbar"><common-header /><insecure-banner /></div>
          <main id="console-main" class="console-main" tabindex="-1"><common-content /></main>
          <footer class="console-footer">
            <span>HomeWaf · 网站与家庭服务防护</span><RouterLink to="/help">使用与许可说明</RouterLink>
          </footer>
        </div>
      </div>
      <setting />
      <ai-assistant />
    </template>
  </div>
</template>
<script lang="ts">
import { defineComponent } from 'vue';
import { mapGetters } from '@/store/helpers';
import { NotifyPlugin } from '@/ui';
import { ElButton } from 'element-plus';
import InsecureBanner from './components/InsecureBanner.vue';
import CommonHeader from './components/Header.vue';
import CommonContent from './components/Content.vue';
import SideNav from './components/SideNav.vue';
import AiAssistant from './components/AiAssistant.vue';
import Setting from './setting.vue';
export default defineComponent({
  components: { InsecureBanner, CommonHeader, CommonContent, SideNav, AiAssistant, Setting },
  data: () => ({ mobile: window.matchMedia('(max-width: 760px)').matches, media: null as MediaQueryList | null }),
  computed: {
    ...mapGetters({ menuRouters: 'permission/routers' }),
    setting() {
      return this.$store.state.setting;
    },
  },
  watch: {
    '$route.path'() {
      if (this.mobile) this.closeMobileNavigation();
    },
    'setting.isSidebarCompact'(closed) {
      if (this.mobile && !closed)
        this.$nextTick(() => this.$el.querySelector('.console-sidebar a, .console-sidebar button')?.focus());
    },
  },
  mounted() {
    this.media = window.matchMedia('(max-width: 760px)');
    this.media.addEventListener('change', this.onViewportChange);
    this.$store.commit('setting/showSidebarCompact', this.mobile);
    this.showLoginNotice();
  },
  unmounted() {
    this.media?.removeEventListener('change', this.onViewportChange);
  },
  methods: {
    onViewportChange(event: MediaQueryListEvent) {
      this.mobile = event.matches;
      this.$store.commit('setting/showSidebarCompact', event.matches);
    },
    closeMobileNavigation() {
      if (!this.mobile || this.setting.isSidebarCompact) return;
      this.$store.commit('setting/showSidebarCompact', true);
      this.$nextTick(() => this.$el.querySelector('.sidebar-toggle')?.focus());
    },
    trapMobileFocus(event: KeyboardEvent) {
      if (!this.mobile || this.setting.isSidebarCompact) return;
      const controls = [
        ...this.$el.querySelectorAll(
          '.console-sidebar a, .console-sidebar button, .console-sidebar input, .console-sidebar summary',
        ),
      ].filter((item: HTMLElement) => item.getClientRects().length);
      const first = controls[0],
        last = controls.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    },
    showLoginNotice() {
      const raw = sessionStorage.getItem('login_notice');
      if (!raw) return;
      sessionStorage.removeItem('login_notice');

      let notice: any = null;
      try {
        notice = JSON.parse(raw);
      } catch (e) {
        return;
      }
      if (!notice || !notice.current_ip) return;

      const dash = (v: string) => (v && String(v).trim() !== '' ? v : '-');
      const rows: Array<[string, string]> = [
        [this.$t('page.login_notice.current_ip') as string, dash(notice.current_ip)],
        [this.$t('page.login_notice.current_area') as string, dash(notice.current_area)],
        [this.$t('page.login_notice.current_time') as string, dash(notice.current_time)],
      ];
      if (notice.is_changed) {
        rows.push([this.$t('page.login_notice.last_ip') as string, dash(notice.last_ip)]);
        rows.push([this.$t('page.login_notice.last_area') as string, dash(notice.last_area)]);
        rows.push([this.$t('page.login_notice.last_time') as string, dash(notice.last_time)]);
      }

      let tipKey = 'page.login_notice.same_tip';
      if (notice.is_changed) tipKey = 'page.login_notice.changed_tip';
      else if (notice.is_first) tipKey = 'page.login_notice.first_tip';

      // 全部用 h() 构造文本节点，IP/归属地这些外部来源的字符串不会被当成 HTML 解析
      const content = (h: any) =>
        h('div', { style: 'font-size:13px;line-height:22px;' }, [
          ...rows.map(([label, value]) =>
            h('div', { style: 'display:flex;gap:8px;' }, [
              h('span', { style: 'flex:0 0 84px;color:var(--td-text-color-secondary);' }, label),
              h('span', { style: 'flex:1;word-break:break-all;' }, value),
            ]),
          ),
          h(
            'div',
            {
              style: `margin-top:8px;${notice.is_changed ? 'color:var(--td-warning-color);' : 'color:var(--td-text-color-secondary);'}`,
            },
            this.$t(tipKey) as string,
          ),
        ]);

      const options: any = {
        title: this.$t(notice.is_changed ? 'page.login_notice.title_changed' : 'page.login_notice.title_normal'),
        content,
        placement: 'bottom-right',
        // 来源变化是要人看到并判断的，不自动消失，必须用户自己关；
        // 一致时 6 秒后自动关掉，别挡住页面
        duration: notice.is_changed ? 0 : 6000,
        closeBtn: true,
      };

      // instance 在 footer 渲染之后才赋值，但点击一定发生在渲染之后，闭包里拿得到
      let instance: any = null;
      if (notice.is_changed) {
        options.footer = (h: any) =>
          h('div', { style: 'display:flex;justify-content:flex-end;' }, [
            h(
              ElButton,
              {
                type: 'primary',
                link: true,
                size: 'small',
                onClick: () => {
                  if (instance) NotifyPlugin.close(instance);
                  this.$router.push('/account/LoginHistory').catch(() => {});
                },
              },
              () => this.$t('page.login_notice.view_history') as string,
            ),
          ]);
      }
      instance = notice.is_changed ? NotifyPlugin.warning(options) : NotifyPlugin.success(options);
    },
  },
});
</script>
