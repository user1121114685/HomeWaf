<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div class="console-header">
    <hw-dialog
      width="600px"
      v-model:visible="update_visible"
      :header="$t('topNav.update.has_new_version')"
      :cancel-btn="{ content: $t('common.cancel'), disabled: isUpdateloading }"
      :show-close="!isUpdateloading"
      :close-on-esc-keydown="!isUpdateloading"
      data-testid="software-update-dialog"
    >
      <template #confirmBtn>
        <!-- 容器环境：应用内升级会在容器重建后回退，这里不给"确认更新"，只给文档入口 -->
        <hw-link
          v-if="!self_update_allowed"
          theme="primary"
          underline
          href="/#/help"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $t('topNav.update.container_doc_link') }}
        </hw-link>
        <hw-button
          v-else
          :theme="update_prerelease ? 'danger' : 'primary'"
          :loading="isUpdateloading"
          :disabled="isUpdateloading"
          @click="handleConfirmUpdate"
        >
          {{ $t('topNav.update.confirm_update') }}
        </hw-button>
      </template>

      <hw-alert v-if="!self_update_allowed" theme="error" :closable="false" :title="$t('topNav.update.container_blocked_title')">
        <template #message>
          <div>{{ $t('topNav.update.container_blocked_tips', { container: container_type || 'container' }) }}</div>
          <div style="margin-top: 8px">
            <strong>{{ $t('topNav.update.container_howto_label') }}</strong>
          </div>
          <pre
            style="
              margin: 4px 0;
              padding: 8px;
              background: rgba(0, 0, 0, 0.05);
              border-radius: 4px;
              white-space: pre-wrap;
            "
          >
Linux / macOS:
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh | bash -s -- --docker

Windows PowerShell:
&amp; ([scriptblock]::Create((irm https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1))) -Docker</pre>
          <div>{{ $t('topNav.update.container_howto_tips') }}</div>
        </template>
      </hw-alert>
      <hw-alert v-else theme="warning" :closable="false">
        <template #message>
          {{ $t('topNav.update.update_danger_tips') }}
        </template>
      </hw-alert>
      <hw-alert v-if="update_error" theme="error" :closable="false" :title="update_error" role="alert" />
      <hw-alert v-if="update_check_warning" theme="warning" :closable="false" :title="update_check_warning" />
      <hw-alert v-if="update_notes_warning" theme="warning" :closable="false" :title="update_notes_warning" />
      <div style="margin: 16px 0 0 0; line-height: 1.8">
        <div>
          <strong>{{ $t('topNav.update.version_label') }}</strong>
          <template v-if="update_current_ver">{{ update_current_ver }} → </template>{{ update_new_ver }}
        </div>
        <span style="color: red" v-if="update_prerelease">
          {{ $t('topNav.update.beta_version_warning') }}
        </span>

        <div>
          <strong>{{ $t('topNav.update.desc_label') }}</strong>
          <section v-for="notes in updateNotes" :key="notes.version" class="software-update-notes">
            <h4>{{ notes.version }}</h4>
            <ul v-if="notes.items.length">
              <li v-for="item in notes.items" :key="item">{{ item }}</li>
            </ul>
            <p v-else>{{ $t('topNav.update.notes_empty') }}</p>
          </section>
        </div>
      </div>
    </hw-dialog>

    <!-- 升级进度：弹窗内联展示 + 后台运行悬浮球，独立于上面的"有新版本"弹窗存活 -->
    <update-progress ref="updateProgress" @retry="handleDoUpdate" @open-rollback="openRollbackDialog" />

    <!-- 版本回退对话框 -->
    <hw-dialog
      width="680px"
      v-model:visible="rollback_visible"
      header="版本回退"
      :confirm-btn="null"
      :cancel-btn="null"
    >
      <hw-alert theme="warning" style="margin-bottom: 16px">
        <template #message>回退后服务将自动重启，请确认操作</template>
      </hw-alert>
      <div v-if="rollback_loading" style="text-align: center; padding: 24px; color: #646a73">加载中...</div>
      <hw-table
        v-else
        :data="rollback_list"
        :columns="rollbackColumns"
        row-key="file_name"
        size="small"
        :selected-row-keys="rollback_selected ? [rollback_selected.file_name] : []"
        @select-change="onRollbackSelect"
        select-on-row-click
      />
      <div style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 16px">
        <hw-button theme="default" @click="rollback_visible = false">取消</hw-button>
        <hw-button
          theme="danger"
          :disabled="!rollback_selected || isRollbackLoading"
          :loading="isRollbackLoading"
          @click="handleConfirmRollback"
        >
          确认回退到 {{ rollback_selected ? rollback_selected.version : '' }}
        </hw-button>
      </div>
    </hw-dialog>

    <!-- 微信公众号二维码对话框 -->

    <div class="console-header-bar">
      <div class="header-operate-left">
        <hw-button
          theme="default"
          shape="square"
          variant="text"
          class="sidebar-toggle"
          @click="changeCollapsed"
          :aria-expanded="!$store.state.setting.isSidebarCompact"
          aria-label="展开或收起导航"
        >
          <Icon :name="$store.state.setting.isSidebarCompact ? 'menu-unfold' : 'menu-fold'" :size="20" />
        </hw-button>
        <div class="header-context">
          <span class="header-context-group">HomeWaf</span><span class="header-context-separator">/</span
          ><span class="header-context-current">{{ pageTitle }}</span>
        </div>
      </div>
      <div class="operations-container">
        <router-link to="/dashboard/screen" target="_blank" rel="noopener noreferrer" class="screen-entry">{{
          $t('menu.dashboard.screen_title')
        }}</router-link>
        <!-- 系统监控 -->
        <system-monitor />
        <!-- 搜索框 -->
        <search />
        <!-- 全局通知 -->
        <notice />
        <!-- 版本说明 -->
        <hw-tooltip placement="bottom" :content="$t('topNav.update.has_new_version')" v-show="hasNewVersion">
          <hw-button
            class="header-update-btn"
            theme="default"
            shape="square"
            variant="text"
            :aria-label="$t('topNav.update.has_new_version')"
            @click="checkVersion('manual')"
            style="position: relative; overflow: visible"
          >
            <NotificationErrorIcon />
            <span
              v-if="update_prerelease"
              style="
                position: absolute;
                top: -6px;
                right: -18px;
                font-size: 10px;
                color: white;
                background: #ff4d4f;
                border-radius: 8px;
                padding: 0 4px;
                line-height: 16px;
                font-weight: bold;
                box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
                white-space: nowrap;
                z-index: 10;
              "
            >
              {{ $t('topNav.update.prerelease_label') }}
            </span>
            <span
              v-else-if="hasNewVersion"
              style="
                position: absolute;
                top: -6px;
                right: -18px;
                font-size: 10px;
                color: white;
                background: #52c41a;
                border-radius: 8px;
                padding: 0 4px;
                line-height: 16px;
                font-weight: bold;
                box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
                white-space: nowrap;
                z-index: 10;
              "
            >
              New
            </span>
          </hw-button>
        </hw-tooltip>

        <hw-tooltip placement="bottom" :content="$t('topNav.help_document')">
          <hw-button
            class="header-help-btn"
            theme="default"
            shape="square"
            variant="text"
            :aria-label="$t('topNav.help_document')"
            @click="navToHelper"
          >
            <help-circle-icon />
          </hw-button>
        </hw-tooltip>
        <hw-dropdown :min-column-width="120" trigger="click">
          <template #dropdown>
            <hw-dropdown-menu>
              <hw-dropdown-item
                v-for="item in langOptions"
                :key="item.value"
                :value="item.value"
                :active="item.value === langValue"
                @click="changeLangEvent(item.value)"
              >
                {{ item.label }}
              </hw-dropdown-item>
            </hw-dropdown-menu>
          </template>
          <hw-button
            theme="default"
            shape="square"
            variant="text"
            class="lang-btn"
            :title="$t('common.language')"
            :aria-label="$t('common.language')"
          >
            <translate-icon :size="20" />
          </hw-button>
        </hw-dropdown>
        <hw-dropdown :min-column-width="125" trigger="click">
          <template #dropdown>
            <hw-dropdown-menu>
              <hw-dropdown-item @click="navToHelper"
                ><help-circle-icon />{{ $t('topNav.help_document') }}</hw-dropdown-item
              >
              <hw-dropdown-item
                v-for="item in langOptions"
                :key="`account-${item.value}`"
                @click="changeLangEvent(item.value)"
                ><translate-icon />{{ item.label }}</hw-dropdown-item
              >
              <hw-dropdown-item class="operations-dropdown-container-item" @click="handleNav('/user/index')">
                <user-circle-icon /> {{ $t('topNav.dropdown_person_center') }}
              </hw-dropdown-item>
              <hw-dropdown-item class="operations-dropdown-container-item" @click="showChangePwd = true">
                <lock-on-icon /> {{ $t('topNav.dropdown_change_password') }}
              </hw-dropdown-item>
              <hw-dropdown-item
                class="operations-dropdown-container-item"
                :disabled="isUpdateloading"
                @click="checkVersion('manual')"
              >
                <RotateIcon /> {{ $t('topNav.dropdown_update') }}
              </hw-dropdown-item>
              <hw-dropdown-item class="operations-dropdown-container-item" @click="openRollbackDialog">
                <RollbackIcon /> 版本回退
              </hw-dropdown-item>
              <hw-dropdown-item
                class="operations-dropdown-container-item"
                :disabled="isResetloading"
                @click="resetServer"
              >
                <ArrowUpDownCircleIcon />{{ $t('topNav.dropdown_reboot_waf') }}
              </hw-dropdown-item>
              <hw-dropdown-item class="operations-dropdown-container-item" @click="handleLogout">
                <poweroff-icon />{{ $t('topNav.dropdown_logout') }}
              </hw-dropdown-item>
            </hw-dropdown-menu>
          </template>
          <hw-button class="header-user-btn" theme="default" variant="text" aria-label="账户菜单">
            <template #icon>
              <user-circle-icon class="header-user-avatar" />
            </template>
            <div class="header-user-account">
              {{ current_account }}
              <chevron-down-icon />
            </div>
          </hw-button>
        </hw-dropdown>
        <hw-tooltip placement="bottom" :content="$t('topNav.system_config')">
          <hw-button
            theme="default"
            shape="square"
            variant="text"
            :aria-label="$t('topNav.system_config')"
            @click="toggleSettingPanel"
          >
            <setting-icon />
          </hw-button>
        </hw-tooltip>
      </div>
    </div>

    <!-- 个人自助修改密码 -->
    <change-password-dialog v-model:visible="showChangePwd" :forced="false" />
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

import {
  LogoGithubIcon,
  HelpCircleIcon,
  UserCircleIcon,
  PoweroffIcon,
  SettingIcon,
  ChevronDownIcon,
  RotateIcon,
  Icon,
  MailIcon,
  NotificationErrorIcon,
  ArrowUpDownCircleIcon,
  LogoWechatStrokeIcon,
  RollbackIcon,
  LockOnIcon,
  TranslateIcon,
} from '@/ui/icons';
import { CheckVersionApi, DoUpdateApi, GetRollbackListApi, DoRollbackApi } from '@/apis/sysinfo';
import { RELEASE_NOTICE_PREFIX } from '@/constants';
import { releaseNoteGroups, type ReleaseNoteGroup } from '@/utils/releaseNotes';

import Notice from './Notice.vue';
import Search from './Search.vue';
import SystemMonitor from './SystemMonitor.vue';
import UpdateProgress from './UpdateProgress.vue';
import ChangePasswordDialog from '@/pages/waf/account/components/ChangePasswordDialog.vue';
import { logoutapi } from '@/apis/login';
import { taskTitles } from '@/ui/workspace';
export default defineComponent({
  components: {
    Notice,
    Search,
    SystemMonitor,
    UpdateProgress,
    ChangePasswordDialog,
    LogoGithubIcon,
    HelpCircleIcon,
    UserCircleIcon,
    PoweroffIcon,
    SettingIcon,
    ChevronDownIcon,
    RotateIcon,
    Icon,
    MailIcon,
    NotificationErrorIcon,
    ArrowUpDownCircleIcon,
    LogoWechatStrokeIcon,
    RollbackIcon,
    LockOnIcon,
    TranslateIcon,
  },
  data() {
    return {
      visibleNotice: false,
      isSearchFocus: false,
      isResetloading: false,
      /**个人修改密码**/
      showChangePwd: false,
      /**更新内容**/
      hasNewVersion: false, //是否有新版本
      checkingVersion: '' as '' | 'auto' | 'manual',
      versionCheckId: 0,
      versionRefreshTimer: 0,
      lastPromptedVersion: '',
      isUpdateloading: false,
      update_visible: false,
      update_new_ver: '',
      update_current_ver: '',
      update_prerelease: false,
      update_channel: 'official',
      update_desc: '',
      update_notes: [] as ReleaseNoteGroup[],
      update_notes_warning: '',
      update_check_warning: '',
      update_error: '',
      /**运行环境：容器类型(空=非容器)与是否允许应用内升级。
       * 容器里升级只对当前容器有效，容器重建就回退到镜像版本，而数据库回不去，
       * 所以容器环境下后端会直接拒绝升级，前端这里换成镜像更新指引。**/
      container_type: '',
      self_update_allowed: true,
      current_account: 'not login',
      /**微信二维码对话框**/
      wechat_visible: false,
      /**版本回退**/
      rollback_visible: false,
      rollback_loading: false,
      isRollbackLoading: false,
      rollback_list: [],
      rollback_selected: null,
      rollbackColumns: [
        { colKey: 'row-select', type: 'single', width: 48 },
        { colKey: 'version', title: '版本', width: 120 },
        {
          colKey: 'backup_time',
          title: '备份时间',
          width: 180,
          cell: (h, { row }) => new Date(row.backup_time).toLocaleString('zh-CN'),
        },
        {
          colKey: 'file_size',
          title: '大小',
          width: 90,
          cell: (h, { row }) => (row.file_size / 1024 / 1024).toFixed(2) + ' MB',
        },
        { colKey: 'file_name', title: '文件名' },
      ],
      /**语言问题**/
      langValue: 'zh_CN',
      langOptions: [
        {
          value: 'zh_CN',
          label: '中文',
        },
        {
          value: 'en_US',
          label: 'English',
        },
      ],
    };
  },
  computed: {
    pageTitle() {
      return taskTitles[String(this.$route.name)]?.[this.$store.state.language.currentLanguage === 'en_US' ? 1 : 0] || this.$t(String(this.$route.meta.title || '控制台'));
    },
    updateNotes() {
      return releaseNoteGroups(this.update_new_ver, this.update_desc, this.update_notes);
    },
  },
  mounted() {
    this.init();
    this.refreshReleaseNotice();
    this.$bus.$on('software-update-available', this.refreshReleaseNotice);
    document.addEventListener('visibilitychange', this.refreshReleaseNotice);
    // Recover a missed WebSocket event using the instance's saved result.
    this.versionRefreshTimer = window.setInterval(this.refreshReleaseNotice, 5 * 60 * 1000);
    // 升级中刷新了页面：把进度捡回来，别让用户以为升级断了
    this.$nextTick(() => {
      (this.$refs.updateProgress as any)?.resume();
    });
  },
  beforeUnmount() {
    ++this.versionCheckId;
    window.clearInterval(this.versionRefreshTimer);
    document.removeEventListener('visibilitychange', this.refreshReleaseNotice);
    this.$bus.$off('software-update-available', this.refreshReleaseNotice);
  },
  methods: {
    // 切换语言
    changeLangEvent(value, context) {
      // 兼容 t-dropdown-item 的载荷：可能是字符串，也可能是 option 对象（{ value }）
      const lang = typeof value === 'string' ? value : (value && value.value) || '';
      if (lang !== 'zh_CN' && lang !== 'en_US') return;
      this.langValue = lang;
      this.$i18n.locale = lang;
      localStorage.setItem('lang', lang);
      location.reload(); // 刷新页面
    },
    //init
    init() {
      //帐号初始化
      this.current_account = localStorage.getItem('current_account');
      //多语言
      this.langValue = localStorage.getItem('lang') || 'zh_CN';
    },
    toggleSettingPanel() {
      this.$store.commit('setting/toggleSettingPanel', true);
    },
    handleLogout() {
      let current_account_token = localStorage.getItem('account_token') ? localStorage.getItem('account_token') : '';
      logoutapi({ account_token: current_account_token }).then((res) => {
        if (res.code == 0) {
          this.$router.replace({ path: '/login', query: { redirect: this.$route.fullPath } });
        } else {
          this.$message.warning(res.msg);
        }
      });
    },
    changeCollapsed() {
      this.$store.commit('setting/toggleSidebarCompact');
    },
    handleNav(url) {
      this.$router.push(url);
    },
    navToHelper() {
      window.open(this.homewafglobalconfig.getOnlineUrl(), '_blank', 'noopener,noreferrer');
    },
    async resetServer() {
      if (this.isResetloading) return;
      this.isResetloading = true;
      try {
        const result = await this.$request.post('/pipeline/reload', {});
        if (result.code === 0) this.$message.success(result.msg);
        else this.$message.warning(result.msg);
      } catch (error) {
        this.$message.warning(error instanceof Error ? error.message : String(error));
      } finally {
        this.isResetloading = false;
      }
    },
    sendMail() {
      window.location.href = `/#/help`;
    },
    openWechat() {
      this.$router.push('/help');
    },
    refreshReleaseNotice() {
      if (!document.hidden && !this.checkingVersion && !this.update_visible) this.checkVersion('auto');
    },
    async checkVersion(method: 'auto' | 'manual') {
      if (this.isUpdateloading || this.checkingVersion === 'manual') return;
      const manual = method === 'manual';
      const id = ++this.versionCheckId;
      this.checkingVersion = method;
      try {
        const res = await CheckVersionApi(manual ? undefined : { cached: 1 }, manual ? undefined : { background: true });
        if (id !== this.versionCheckId) return;
        if (res.code !== 0) {
          if (manual) this.$message.warning(res.msg);
          return;
        }
        const data = res.data;
        this.hasNewVersion = data.need_update === true && !!data.version_new;
        this.update_new_ver = this.hasNewVersion ? data.version_new : '';
        this.update_current_ver = data.version || '';
        this.update_desc = data.version_desc || '';
        this.update_notes = data.version_notes || [];
        this.update_notes_warning = data.version_notes_warning || '';
        this.update_check_warning = data.check_warning || '';
        this.update_error = '';
        this.container_type = data.container || '';
        this.self_update_allowed = data.self_update_allowed !== false;
        this.update_prerelease = data.prerelease ?? this.update_new_ver.split('+')[0].includes('-');
        this.update_channel = data.update_channel || (this.update_prerelease ? 'github' : 'official');
        if (!this.hasNewVersion) {
          this.update_visible = false;
          if (manual) this.$message.info(res.msg);
          return;
        }
        if (!manual && (document.hidden || this.update_prerelease)) return;
        let seen = this.lastPromptedVersion === this.update_new_ver;
        if (!this.update_prerelease) {
          const key = RELEASE_NOTICE_PREFIX + JSON.stringify([window.location.pathname, this.current_account]);
          try {
            seen ||= localStorage.getItem(key) === this.update_new_ver;
            if (!seen) localStorage.setItem(key, this.update_new_ver);
          } catch { /* Use the in-memory marker when browser storage is unavailable. */ }
          this.lastPromptedVersion = this.update_new_ver;
        }
        if (manual || !seen) this.update_visible = true;
      } catch {
        if (manual && id === this.versionCheckId) this.$message.warning(this.$t('topNav.update.check_failed'));
      } finally {
        if (id === this.versionCheckId) this.checkingVersion = '';
      }
    },
    handleConfirmUpdate() {
      if (this.isUpdateloading) return;
      // 如果是beta版本，需要二次确认
      if (this.update_prerelease) {
        const confirmDia = this.$dialog.confirm({
          header: this.$t('topNav.update.beta_confirm_title') || '确认更新测试版本',
          body: this.$t('topNav.update.beta_confirm_content') || '您正在更新测试版本，该版本可能不稳定，确定要继续吗？',
          confirmBtn: {
            theme: 'danger',
            content: this.$t('topNav.update.beta_confirm_yes') || '确认更新',
          },
          cancelBtn: {
            theme: 'default',
            content: this.$t('topNav.update.beta_confirm_no') || '取消',
          },
          onConfirm: () => {
            this.handleDoUpdate();
            confirmDia.destroy();
          },
        });
      } else {
        // 非beta版本直接更新
        this.handleDoUpdate();
      }
    },
    openRollbackDialog() {
      this.rollback_visible = true;
      this.rollback_selected = null;
      this.rollback_loading = true;
      GetRollbackListApi()
        .then((res: any) => {
          if (res.code === 0) {
            this.rollback_list = res.data || [];
          } else {
            this.$message.warning(res.msg || '获取备份列表失败');
          }
        })
        .catch(() => {
          this.$message.warning('获取备份列表失败');
        })
        .finally(() => {
          this.rollback_loading = false;
        });
    },
    onRollbackSelect(selectedKeys: any[], { selectedRowData }: any) {
      this.rollback_selected = selectedRowData[0] || null;
    },
    handleConfirmRollback() {
      if (!this.rollback_selected) return;
      const that = this;
      const target = this.rollback_selected;
      const confirmDia = this.$dialog.confirm({
        header: '确认回退版本',
        body: `确定将程序回退到版本 ${target.version}？\n回退后服务将自动重启，请耐心等待。`,
        confirmBtn: { theme: 'danger', content: '确认回退' },
        onConfirm: () => {
          that.isRollbackLoading = true;
          DoRollbackApi({ version: target.version })
            .then((res: any) => {
              if (res.code === 0) {
                confirmDia.destroy();
                that.$message.success(res.msg || '已发起回退，等待重启通知');
                that.rollback_visible = false;
              } else {
                confirmDia.hide();
                that.$message.warning(res.msg || '回退失败');
              }
            })
            .catch(() => {
              confirmDia.hide();
              that.$message.warning('回退请求失败');
            })
            .finally(() => {
              that.isRollbackLoading = false;
            });
        },
      });
    },
    handleDoUpdate() {
      //处理升级
      let that = this;
      if (that.isUpdateloading) return;
      // 容器环境后端会直接拒绝，这里先拦一道，避免无谓请求和误导性的 loading
      if (!that.self_update_allowed) {
        that.$message.warning(that.$t('topNav.update.container_blocked_title'));
        return;
      }
      that.isUpdateloading = true;
      that.update_error = '';
      DoUpdateApi({ channel: that.update_channel })
        .then((res) => {
          let resdata = res;
          console.log(resdata);
          if (resdata.code === 0) {
            // 弹窗不再一关了之：换成进度面板，用户能看见跑到哪一步
            that.update_visible = false;
            that.isUpdateloading = false;
            (that.$refs.updateProgress as any)?.start();
          } else {
            that.update_error = resdata.msg || that.$t('topNav.update.request_failed');
            that.$message.warning(resdata.msg);
            // 升级失败，关闭加载并显示错误信息
            that.isUpdateloading = false;
          }
        })
        .catch((e: Error) => {
          // 请求异常，关闭加载并显示错误信息
          that.isUpdateloading = false;
          that.update_error = that.$t('topNav.update.request_failed');
          console.log(e);
        });
    },
  },
});
</script>
