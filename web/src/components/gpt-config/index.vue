<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <!-- AI(GPT) 参数设置弹窗：规则编辑页、AI 助手等入口共用一份，避免各处各写一遍 -->
  <hw-dialog
    :header="$t('page.gpt.config.title')"
    :visible="visible"
    width="760px"
    :footer="false"
    @update:visible="(val) => $emit('update:visible', val)"
  >
    <hw-loading :loading="loading" size="small">
      <!-- autocomplete 关掉：URL/模型文本框 + Token 密码框会被浏览器当成登录表单，填进 admin 口令 -->
      <hw-form :data="gptConfig" :label-width="110" @submit.prevent autocomplete="off">
        <hw-form-item :label="$t('page.gpt.config.url')" name="gpt_url">
          <hw-input v-model="gptConfig.gpt_url" :placeholder="'https://api.deepseek.com'" autocomplete="off" />
        </hw-form-item>
        <hw-form-item :label="$t('page.gpt.config.model')" name="gpt_model">
          <hw-input v-model="gptConfig.gpt_model" :placeholder="'deepseek-chat'" autocomplete="off" />
        </hw-form-item>
        <hw-form-item :label="$t('page.gpt.config.token')" name="gpt_token">
          <hw-input
            v-model="gptConfig.gpt_token"
            type="password"
            autocomplete="new-password"
            :disabled="tokenCleared"
            :placeholder="
              tokenCleared
                ? $t('page.gpt.config.token_will_clear')
                : gptConfig.has_token
                ? $t('page.gpt.config.token_set_placeholder')
                : $t('page.gpt.config.token_empty_placeholder')
            "
          />
        </hw-form-item>
      </hw-form>
      <div class="gpt-token-tip">
        <span>{{ $t('page.gpt.config.token_tip') }}</span>
        <button type="button" v-if="gptConfig.has_token && !tokenCleared" class="t-button-link gpt-token-clear" @click="onClickClearToken">
          {{ $t('page.gpt.config.token_clear') }}
        </button>
        <button type="button" v-if="tokenCleared" class="t-button-link gpt-token-clear" @click="onClickCancelClearToken">
          {{ $t('page.gpt.config.token_cancel_clear') }}
        </button>
      </div>

      <div class="gpt-preset-title">{{ $t('page.gpt.config.preset_title') }}</div>
      <div class="gpt-preset-list">
        <div class="gpt-preset-item" v-for="(p, pIndex) in gptProviders" :key="pIndex">
          <div class="gpt-preset-main">
            <span class="gpt-preset-name">{{ p.name }}</span>
            <span class="gpt-preset-desc">{{ gptPresetDesc(p) }}</span>
          </div>
          <div class="gpt-preset-url">{{ p.url }}</div>
          <div class="gpt-preset-models">
            <hw-tag
              v-for="(m, mIndex) in p.models"
              :key="mIndex"
              size="small"
              variant="light"
              class="gpt-preset-model"
              @click="applyGptPreset(p, m)"
            >{{ m }}</hw-tag>
          </div>
          <div class="gpt-preset-ops">
            <hw-button size="small" variant="outline" @click="applyGptPreset(p)">{{ $t('page.gpt.config.preset_use') }}</hw-button>
            <hw-link theme="primary" hover="color" size="small" :href="p.home" target="_blank" rel="noopener noreferrer">{{ $t('page.gpt.config.preset_apply_key') }}</hw-link>
          </div>
        </div>
      </div>

      <div class="gpt-config-ops">
        <hw-button theme="primary" :loading="saving" @click="saveGptConfig">{{ $t('common.submit') }}</hw-button>
        <hw-button variant="outline" @click="$emit('update:visible', false)">{{ $t('common.close') }}</hw-button>
      </div>
    </hw-loading>
  </hw-dialog>
</template>

<script>
import { wafGptConfigGetApi, wafGptConfigSaveApi } from '@/apis/gpt';
import { GPT_PROVIDERS } from '@/utils/gptProviders';

// 与后端 waf_service.ConfigClearSentinel 约定：密钥留空=保留原值，
// 要真正清空必须提交这个哨兵值。
const TOKEN_CLEAR_SENTINEL = '__SAMWAF_CLEAR__';

export default {
  name: 'GptConfigDialog',
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      loading: false,
      saving: false,
      gptConfig: { gpt_url: '', gpt_model: '', gpt_token: '', has_token: false },
      tokenCleared: false, // 本次编辑是否点了「清空密钥」
      gptProviders: GPT_PROVIDERS,
    };
  },
  watch: {
    visible(val) {
      if (val) {
        this.loadGptConfig();
      }
    },
  },
  mounted() {
    if (this.visible) {
      this.loadGptConfig();
    }
  },
  methods: {
    // 拉取当前配置（密钥不回传明文，只回 has_token）
    async loadGptConfig() {
      this.loading = true;
      this.gptConfig.gpt_token = '';
      this.tokenCleared = false;
      try {
        const res = await wafGptConfigGetApi();
        if (res.code === 0 && res.data) {
          this.gptConfig.gpt_url = res.data.gpt_url || '';
          this.gptConfig.gpt_model = res.data.gpt_model || '';
          this.gptConfig.has_token = !!res.data.has_token;
        }
      } catch (e) {
        console.log(e);
      } finally {
        this.loading = false;
      }
    },
    // 预设服务商说明（按界面语言）
    gptPresetDesc(p) {
      const en = ((this.$i18n && this.$i18n.locale) || '').toLowerCase().indexOf('en') === 0;
      return en ? p.en : p.zh;
    },
    // 应用预设：填 url + 选定/默认模型
    applyGptPreset(p, model) {
      this.gptConfig.gpt_url = p.url;
      this.gptConfig.gpt_model = model || (p.models && p.models[0]) || '';
    },
    // 清空密钥：留空只表示“保留原值”，真要清空得显式提交哨兵值
    onClickClearToken() {
      this.tokenCleared = true;
      this.gptConfig.gpt_token = '';
    },
    onClickCancelClearToken() {
      this.tokenCleared = false;
      this.gptConfig.gpt_token = '';
    },
    // 保存 GPT 参数（token 留空=保留原密钥，点过清空=提交哨兵真正清掉）
    async saveGptConfig() {
      if (!this.gptConfig.gpt_url || !this.gptConfig.gpt_model) {
        this.$message.warning(this.$t('page.gpt.config.url_model_required'));
        return;
      }
      this.saving = true;
      try {
        const typedToken = (this.gptConfig.gpt_token || '').trim();
        const res = await wafGptConfigSaveApi({
          gpt_url: this.gptConfig.gpt_url.trim(),
          gpt_model: this.gptConfig.gpt_model.trim(),
          gpt_token: this.tokenCleared ? TOKEN_CLEAR_SENTINEL : typedToken,
        });
        if (res.code === 0) {
          this.$message.success(this.$t('page.gpt.config.save_ok'));
          if (this.tokenCleared) {
            this.gptConfig.has_token = false;
          } else if (typedToken) {
            this.gptConfig.has_token = true;
          }
          this.tokenCleared = false;
          this.gptConfig.gpt_token = '';
          this.$emit('update:visible', false);
          // 通知调用方刷新"是否已配置"状态
          this.$emit('saved', { has_token: this.gptConfig.has_token });
        } else {
          this.$message.warning(res.msg || this.$t('page.gpt.config.save_fail'));
        }
      } catch (e) {
        console.log(e);
        this.$message.error(this.$t('page.gpt.config.save_fail'));
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.gpt-token-tip {
  color: var(--td-text-color-placeholder, #999);
  font-size: 13px;
  margin: 4px 0 12px 110px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.gpt-token-clear {
  flex-shrink: 0;
}
.gpt-preset-title {
  font-weight: 500;
  margin: 8px 0;
}
.gpt-preset-list {
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--td-component-border, #e7e7e7);
  border-radius: 6px;
}
.gpt-preset-item {
  padding: 8px 12px;
  border-bottom: 1px solid var(--td-component-border, #eee);
}
.gpt-preset-item:last-child {
  border-bottom: none;
}
.gpt-preset-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.gpt-preset-name {
  font-weight: 500;
}
.gpt-preset-desc {
  color: var(--td-text-color-secondary, #888);
  font-size: 13px;
}
.gpt-preset-url {
  color: var(--td-text-color-secondary, #666);
  font-size: 13px;
  margin: 2px 0;
  word-break: break-all;
}
.gpt-preset-models {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 4px 0;
}
.gpt-preset-model {
  cursor: pointer;
}
.gpt-preset-ops {
  display: flex;
  align-items: center;
  gap: 12px;
}
.gpt-config-ops {
  margin-top: 12px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
</style>
