<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-drawer
      size="408px"
      :footer="false"
      v-model:visible="showSettingPanel"
      :header="$t('page.right_setting.page_setting')"
      :closeBtn="true"
      :onCloseBtnClick="handleCloseDrawer"
      class="setting-drawer-container"
    >
      <div class="setting-container">
        <hw-tabs default-value="theme" class="setting-tabs">
        <hw-tab-panel value="theme" :label="$t('page.right_setting.tab_theme')">
        <hw-form :data="formData" size="large" ref="form" labelAlign="left" @reset="onReset" @submit="onSubmit">
          <div class="setting-group-title">{{ $t('page.right_setting.theme_mode') }}</div>
          <hw-radio-group v-model="formData.mode" class="setting-theme-modes" :aria-label="$t('page.right_setting.theme_mode')">
            <div v-for="(item, index) in MODE_OPTIONS" :key="index" class="setting-layout-drawer">
              <div>
                <hw-radio-button :key="index" :value="item.type" :aria-label="$t(item.textKey)"
                  ><span class="hw-sr-only">{{ $t(item.textKey) }}</span><component :is="getModeIcon(item.type)"
                /></hw-radio-button>
                <p class="setting-choice-label">{{ $t(item.textKey) }}</p>
              </div>
            </div>
          </hw-radio-group>
          <div class="setting-group-title">{{ $t('page.right_setting.theme_color') }}</div>
          <hw-radio-group v-model="formData.brandTheme" class="setting-theme-colors" :aria-label="$t('page.right_setting.theme_color')">
            <div
              v-for="(item, index) in COLOR_OPTIONS.slice(0, COLOR_OPTIONS.length - 1)"
              :key="index"
              class="setting-layout-drawer"
            >
              <hw-radio-button :key="index" :value="item" class="setting-layout-color-group">
                <color-container :value="item" />
              </hw-radio-button>
            </div>
            <div class="setting-layout-drawer">
              <hw-popup
                destroy-on-close
                expand-animation
                placement="bottom-right"
                trigger="click"
                :visible="isColoPickerDisplay"
                @visible-change="onPopupVisibleChange"
                :overlayStyle="{ padding: 0 }"
              >
                <template #content>
                  <hw-color-picker-panel
                    :on-change="changeColor"
                    :color-modes="['monochrome']"
                    format="HEX"
                    :swatch-colors="[]"
                /></template>
                <hw-radio-button
                  :value="COLOR_OPTIONS[COLOR_OPTIONS.length - 1]"
                  class="setting-layout-color-group dynamic-color-btn"
                >
                  <color-container :value="COLOR_OPTIONS[COLOR_OPTIONS.length - 1]" />
                </hw-radio-button>
              </hw-popup>
            </div>
          </hw-radio-group>
          <div class="setting-group-title">{{ $t('page.right_setting.navigation_layout') }}</div>

          <hw-radio-group v-model="formData.layout" class="setting-navigation-layout" :aria-label="$t('page.right_setting.navigation_layout')">
            <div v-for="(item, index) in LAYOUT_OPTION" :key="index" class="setting-layout-drawer">
              <hw-radio-button :key="index" :value="item" :aria-label="layoutLabel(item)"><thumbnail :url="getThumbnailUrl(item)" /></hw-radio-button>
              <p class="setting-choice-label">{{ layoutLabel(item) }}</p>
            </div>
          </hw-radio-group>

          <hw-form-item v-show="formData.layout === 'mix'" :label="$t('page.right_setting.split_menu')" name="splitMenu">
            <hw-switch v-model="formData.splitMenu"></hw-switch>
          </hw-form-item>

          <hw-form-item v-show="formData.layout !== 'side'" :label="$t('page.right_setting.fix_header')" name="isHeaderFixed">
            <hw-switch v-model="formData.isHeaderFixed"></hw-switch>
          </hw-form-item>
          <hw-form-item v-show="formData.layout !== 'top'" :label="$t('page.right_setting.fix_sidebar')" name="isSidebarFixed">
            <hw-switch v-model="formData.isSidebarFixed"></hw-switch>
          </hw-form-item>

          <div class="setting-group-title">{{ $t('page.right_setting.element_switches') }}</div>
          <hw-form-item :label="$t('page.right_setting.show_header')" name="showHeader" v-show="formData.layout === 'side'">
            <hw-switch v-model="formData.showHeader"></hw-switch>
          </hw-form-item>
          <hw-form-item :label="$t('page.right_setting.show_breadcrumb')" name="showBreadcrumb">
            <hw-switch v-model="formData.showBreadcrumb"></hw-switch>
          </hw-form-item>
          <hw-form-item :label="$t('page.right_setting.show_footer')" name="showFooter">
            <hw-switch v-model="formData.showFooter"></hw-switch>
          </hw-form-item>
          <hw-form-item :label="$t('page.right_setting.use_tabs_router')" name="isUseTabsRouter">
            <hw-switch v-model="formData.isUseTabsRouter"></hw-switch>
          </hw-form-item>
          <hw-form-item
            :label="$t('page.right_setting.footer_position')"
            name="footerPosition"
            v-show="formData.showFooter && !formData.isSidebarFixed"
          >
            <hw-switch v-model="formData.isFooterAside"></hw-switch>
          </hw-form-item>
        </hw-form>
        <!--<div class="setting-info">
          <p>请复制后手动修改配置文件: /src/config/style.ts</p>
          <hw-button theme="primary" variant="text" @click="handleCopy"> 复制配置项 </hw-button>
        </div>-->
        <div class="setting-footer">
          <hw-button theme="default" variant="outline" @click="handleClearSettings">
            {{ $t('page.right_setting.clear_settings') }}
          </hw-button>
        </div>
        </hw-tab-panel>

        <hw-tab-panel value="general" :label="$t('page.right_setting.tab_general')">
          <div class="setting-general">
            <div class="setting-group-title">{{ $t('page.right_setting.local_timeout_title') }}</div>
            <div class="local-timeout-body">
              <hw-input-number
                v-model="localTimeoutSec"
                :min="localTimeoutMinSec"
                :max="localTimeoutMaxSec"
                :step="1"
                theme="column"
                style="width: 150px;"
              />
              <span class="local-timeout-unit">{{ $t('page.right_setting.local_timeout_unit') }}</span>
              <hw-button theme="primary" @click="saveLocalTimeout">{{ $t('common.save') }}</hw-button>
              <hw-button variant="outline" @click="resetLocalTimeout">{{ $t('page.right_setting.local_timeout_reset') }}</hw-button>
            </div>
            <div class="local-timeout-tip">
              {{ $t('page.right_setting.local_timeout_tip', { def: localTimeoutDefaultSec }) }}
            </div>
          </div>
        </hw-tab-panel>
        </hw-tabs>
      </div>
    </hw-drawer>
  </div>
</template>
<script lang="ts">
import { mapGetters } from '@/store/helpers';
import { Color } from '@/ui/color';
import { PopupVisibleChangeContext } from '@/ui';

import STYLE_CONFIG from '@/config/style';
import { insertThemeStylesheet, generateColorMap } from '@/config/color';

import Thumbnail from '@/components/thumbnail/index.vue';
import LayoutSideThumbnail from '@/assets/assets-setting-layout-side.svg?url';
import LayoutTopThumbnail from '@/assets/assets-setting-layout-top.svg?url';
import LayoutMixThumbnail from '@/assets/assets-setting-layout-mix.svg?url';
import ColorContainer from '@/components/color/index.vue';

import SettingDarkIcon from '@/assets/assets-setting-dark.svg';
import SettingLightIcon from '@/assets/assets-setting-light.svg';
import SettingAutoIcon from '@/assets/assets-setting-auto.svg';
import {
  getRequestTimeout, setRequestTimeout, DEFAULT_REQUEST_TIMEOUT, MIN_REQUEST_TIMEOUT, MAX_REQUEST_TIMEOUT
} from '@/config/requestTimeout';

const SETTING_STORAGE_KEY = 'samwaf_page_setting';
const LAYOUT_OPTION = ['side', 'top', 'mix'];

const LAYOUT_THUMBNAIL_MAP = {
  side: LayoutSideThumbnail,
  top: LayoutTopThumbnail,
  mix: LayoutMixThumbnail,
};
const COLOR_OPTIONS = ['default', 'cyan', 'green', 'yellow', 'orange', 'red', 'pink', 'purple', 'dynamic'];
const MODE_OPTIONS = [
  { type: 'light', textKey: 'page.right_setting.theme_mode_color_light' },
  { type: 'dark', textKey: 'page.right_setting.theme_mode_color_dark' },
  { type: 'auto', textKey: 'page.right_setting.theme_mode_color_auto' },
];

export default {
  name: 'DefaultLayoutSetting',
  components: { Thumbnail, ColorContainer },
  data() {
    return {
      colors: {
        hex: null,
      },
      MODE_OPTIONS,
      LAYOUT_OPTION,
      COLOR_OPTIONS,
      visible: false,
      formData: { ...STYLE_CONFIG },
      isColoPickerDisplay: false,
      // 前端请求超时（浏览器本地设置，单位秒）
      localTimeoutSec: Math.round(getRequestTimeout() / 1000),
      localTimeoutMinSec: Math.round(MIN_REQUEST_TIMEOUT / 1000),
      localTimeoutMaxSec: Math.round(MAX_REQUEST_TIMEOUT / 1000),
      localTimeoutDefaultSec: Math.round(DEFAULT_REQUEST_TIMEOUT / 1000),
    };
  },
  computed: {
    ...mapGetters('setting', ['showSettingBtn']),
    showSettingPanel: {
      get() {
        return this.$store.state.setting.showSettingPanel;
      },
      set(newVal) {
        this.$store.commit('setting/toggleSettingPanel', newVal);
      },
    },
    iconName() {
      return this.visible ? 'close' : 'setting';
    },
    showOthers() {
      return (this.formData.showFooter && !this.formData.isSidebarFixed) || !this.formData.splitMenu;
    },
  },
  watch: {
    formData: {
      handler(newVal) {
        // 没有在formData中 需要从store中同步过来
        const { isSidebarCompact } = this.$store.state.setting;
        this.$store.dispatch('setting/changeTheme', { ...newVal, isSidebarCompact });
        // 用户修改后临时保存到 localStorage
        this.saveSettingToStorage(newVal);
      },
      deep: true,
    },
  },
  created() {
    // 从 localStorage 恢复上次保存的设置
    const saved = this.getSettingFromStorage();
    if (saved && Object.keys(saved).length > 0) {
      this.formData = { ...this.formData, ...saved };
      const { isSidebarCompact } = this.$store.state.setting;
      this.$store.dispatch('setting/changeTheme', { ...this.formData, isSidebarCompact });
    }
  },
  mounted() {
    document.querySelector('.dynamic-color-btn')?.addEventListener('click', () => {
      this.isColoPickerDisplay = true;
    });
  },
  methods: {
    // 保存前端请求超时（浏览器本地设置，立即生效，无需刷新）
    saveLocalTimeout() {
      const savedMs = setRequestTimeout(Number(this.localTimeoutSec) * 1000);
      this.localTimeoutSec = Math.round(savedMs / 1000);
      this.$message.success(this.$t('page.right_setting.local_timeout_saved'));
    },
    // 恢复默认超时
    resetLocalTimeout() {
      const savedMs = setRequestTimeout(DEFAULT_REQUEST_TIMEOUT);
      this.localTimeoutSec = Math.round(savedMs / 1000);
      this.$message.success(this.$t('page.right_setting.local_timeout_saved'));
    },
    onPopupVisibleChange(visible: boolean, context: PopupVisibleChangeContext) {
      if (!visible && context.trigger === 'document') this.isColoPickerDisplay = visible;
    },
    onReset(): void {
      this.formData = {
        ...STYLE_CONFIG,
      };
      this.$message.success('已恢复初始设置');
    },
    onSubmit({ result, firstError, e }): void {
      e.preventDefault();
      if (result === true) {
        this.visible = false;
      } else {
        this.$message.warning(firstError);
      }
    },
    getModeIcon(mode: string) {
      if (mode === 'light') {
        return SettingLightIcon;
      }
      if (mode === 'dark') {
        return SettingDarkIcon;
      }
      return SettingAutoIcon;
    },
    getThumbnailUrl(name: string) {
      return LAYOUT_THUMBNAIL_MAP[name] || LayoutSideThumbnail;
    },
    layoutLabel(name: string) {
      const english = this.$store.state.language.currentLanguage === 'en_US';
      return ({ side: english ? 'Sidebar' : '侧边导航', top: english ? 'Top navigation' : '顶部导航', mix: english ? 'Mixed' : '混合导航' })[name];
    },
    handleClick(): void {
      this.$store.commit('setting/toggleSettingPanel', true);
    },
    handleCloseDrawer(): void {
      this.$store.commit('setting/toggleSettingPanel', false);
    },
    handleCopy(): void {
      const text = JSON.stringify(this.formData, null, 4);
      this.$copyText(text).then(() => {
        this.$message.closeAll();
        this.$message.success('复制成功');
      });
    },
    changeColor(hex: string) {
      const { setting } = this.$store.state;

      // hex 主题色
      const newPalette = Color.getPaletteByGradation({
        colors: [hex],
        step: 10,
      })[0];
      const { mode } = this.$store.state.setting;
      const colorMap = generateColorMap(hex, newPalette, mode);

      this.$store.commit('setting/addColor', { [hex]: colorMap });

      insertThemeStylesheet(hex, colorMap, mode);

      this.$store.dispatch('setting/changeTheme', { ...setting, brandTheme: hex });
    },
    getSettingFromStorage() {
      try {
        const raw = localStorage.getItem(SETTING_STORAGE_KEY);
        return raw ? JSON.parse(raw) : null;
      } catch {
        return null;
      }
    },
    saveSettingToStorage(data: Record<string, unknown>) {
      try {
        const payload = { ...STYLE_CONFIG, ...data };
        const toSave = {
          showFooter: payload.showFooter,
          isSidebarCompact: payload.isSidebarCompact,
          showBreadcrumb: payload.showBreadcrumb,
          mode: payload.mode,
          layout: payload.layout,
          splitMenu: payload.splitMenu,
          isFooterAside: payload.isFooterAside,
          isSidebarFixed: payload.isSidebarFixed,
          isHeaderFixed: payload.isHeaderFixed,
          showHeader: payload.showHeader,
          backgroundTheme: payload.backgroundTheme,
          brandTheme: payload.brandTheme,
          isUseTabsRouter: payload.isUseTabsRouter,
        };
        localStorage.setItem(SETTING_STORAGE_KEY, JSON.stringify(toSave));
      } catch (_) {
        // ignore
      }
    },
    handleClearSettings() {
      try {
        localStorage.removeItem(SETTING_STORAGE_KEY);
      } catch (_) {
        // ignore
      }
      this.formData = { ...STYLE_CONFIG };
      const { isSidebarCompact } = this.$store.state.setting;
      this.$store.dispatch('setting/changeTheme', { ...this.formData, isSidebarCompact });
      this.$message.success(this.$t('page.right_setting.clear_settings_success'));
    },
  },
};
</script>
<style lang="less">
@import '@/style/variables.less';

.homewaf-setting {
  z-index: 100;
  position: fixed;
  bottom: 200px;
  right: 0;
  transition: transform 0.3s cubic-bezier(0.7, 0.3, 0.1, 1), visibility 0.3s cubic-bezier(0.7, 0.3, 0.1, 1);
  height: 40px;
  width: 40px;
  border-radius: 20px 0 0 20px;
  transition: all 0.3s;

  .t-icon {
    margin-left: 8px;
  }

  .homewaf-setting-text {
    font-size: 12px;
    display: none;
  }

  &:hover {
    width: 96px;

    .homewaf-setting-text {
      display: inline-block;
    }
  }
}

.setting-layout-color-group {
  display: inline-flex;
  justify-content: center;
  align-items: center;
}

.homewaf-setting-close {
  position: fixed;
  bottom: 200px;
  right: 300px;
}

.setting-group-title {
  font-size: 14px;
  line-height: 22px;
  margin: 32px 0 24px 0;
  text-align: left;
  font-family: PingFang SC;
  font-style: normal;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.setting-general {
  .local-timeout-body {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .local-timeout-unit {
    color: var(--td-text-color-secondary);
  }

  .local-timeout-tip {
    color: var(--td-text-color-placeholder);
    font-size: 12px;
    margin-top: 12px;
    line-height: 1.6;
  }
}

.setting-group-color {
  position: relative;

  > div {
    position: absolute;
    z-index: 2;
    right: 0;
  }
}

.setting-link {
  cursor: pointer;
  color: var(--td-brand-color);
  margin-bottom: 8px;
}

.setting-info {
  position: absolute;
  padding: 24px;
  bottom: 0;
  left: 0;
  line-height: 20px;
  font-size: 12px;
  text-align: center;
  color: var(--td-text-color-placeholder);
  width: 100%;
  background: var(--td-bg-color-container);
}

.setting-drawer-container {
  .setting-container {
    padding-bottom: 24px;
  }

  .setting-footer {
    margin-top: 24px;
    padding-top: 16px;
    border-top: 1px solid var(--td-component-border);
  }

  .el-radio-group {
    min-height: 32px;
    width: 100%;
    height: auto;
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
    align-items: flex-start;
  }
  .setting-theme-modes,
  .setting-navigation-layout {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .setting-choice-label {
    margin: 8px 0 0;
    text-align: center;
    color: var(--el-text-color-regular);
    font-size: 13px;
    line-height: 22px;
  }
  .setting-theme-colors { gap: 8px; }
  .setting-theme-modes svg { display: block; width: 88px; height: 48px; max-width: 100%; }
  .setting-navigation-layout img { display: block; max-width: 100%; object-fit: contain; }

  .setting-layout-drawer {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 0;
    > div { width: 100%; }

    .el-radio-button {
      --el-radio-button-checked-bg-color: var(--el-color-primary-light-9);
      --el-radio-button-checked-text-color: var(--el-text-color-primary);
      display: inline-flex;
      width: 100%;
      height: auto;
    }
    .el-radio-button > .el-radio-button__inner {
      display: flex; align-items: center; justify-content: center;
      width: 100%; height: auto; padding: 6px;
      border-radius: 4px; border: 2px solid var(--el-border-color);
      background: var(--el-bg-color); box-shadow: none; outline: none;
    }
    .el-radio-button.is-active .el-radio-button__inner {
      border-color: var(--el-color-primary);
      background: var(--el-color-primary-light-9); box-shadow: none;
    }
    .el-radio-button:focus-within .el-radio-button__inner {
      outline: 2px solid var(--el-color-primary); outline-offset: 2px;
    }
  }
  .setting-layout-drawer .setting-layout-color-group > .el-radio-button__inner {
    border-radius: 50%; width: 36px; height: 36px; padding: 4px;
  }
  .el-form-item { align-items: center; margin-bottom: 18px; }
  .el-form-item__label {
    flex: 1; width: auto !important; height: auto; line-height: 22px;
    justify-content: flex-start; color: var(--el-text-color-regular);
  }
  .el-form-item__content { flex: 0 0 auto; margin-left: 0 !important; justify-content: flex-end; }
  .el-drawer__body { padding: 16px 24px; }
  @media (max-width: 450px) {
    max-width: 100vw;
    .el-drawer__body { padding: 12px 16px; }
    .setting-theme-modes, .setting-navigation-layout { gap: 8px; }
  }
}

.setting-route-theme {
  .t-form__label {
    min-width: 310px !important;
    color: var(--td-text-color-secondary);
  }
}

.setting-color-theme {
  .setting-layout-drawer {
    .t-radio-button {
      height: 32px;
    }

    &:last-child {
      margin-right: 0;
    }
  }
}
</style>
