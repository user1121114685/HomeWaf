<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="ssl-form">
    <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="350">
      <!-- 编辑模式下显示有效期 -->
      <hw-form-item :label="$t('page.ssl.label_domains')" name="domains" v-if="isEdit">
        <span>{{formData.domains}}</span>
      </hw-form-item>
      <hw-form-item :label="$t('page.ssl.label_valid_to')" name="valid_to" v-if="isEdit && formData.valid_to">
        <span>{{formData.valid_to}} ({{formData.expiration_info}})</span>
      </hw-form-item>
      <hw-form-item :label="$t('page.ssl.label_bind_hosts')" name="bind_hosts" v-if="isEdit && formData.bind_hosts && formData.bind_hosts.length > 0">
        <div>
          <div v-for="(host, index) in formData.bind_hosts" :key="index" style="margin-bottom: 4px;">
            {{ host }}
          </div>
        </div>
      </hw-form-item>

      <hw-form-item :label="$t('page.ssl.label_cert_content')" name="cert_content">
        <hw-textarea v-model="formData.cert_content" :style="{ width: 'min(100%, 480px)' }" rows="4" ></hw-textarea>
      </hw-form-item>

      <hw-form-item :label="$t('page.ssl.label_key_content')" name="key_content">
        <hw-textarea v-model="formData.key_content" :style="{ width: 'min(100%, 480px)' }" rows="4" ></hw-textarea>
      </hw-form-item>

      <!-- 编辑模式下显示自动更新相关字段 -->
      <template v-if="isEdit">
        <hw-form-item>
          <b>{{$t("page.ssl.label_auto_tip")}}</b>
        </hw-form-item>
        <hw-form-item :label="$t('page.ssl.label_auto_crt_path')" name="cert_path">
          <hw-textarea v-model="formData.cert_path" :style="{ width: 'min(100%, 480px)' }" rows="4"></hw-textarea>
        </hw-form-item>
        <hw-form-item :label="$t('page.ssl.label_auto_key_path')" name="key_path">
          <hw-textarea v-model="formData.key_path" :style="{ width: 'min(100%, 480px)' }" rows="4"></hw-textarea>
        </hw-form-item>
      </template>

      <hw-form-item style="float: right;margin-top:5px">
        <hw-button variant="outline" @click="$emit('close')">{{ $t('common.close') }}</hw-button>
        <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
      </hw-form-item>
    </hw-form>
  </div>
</template>

<script>
import {defineComponent} from 'vue';



export default defineComponent({
  name: 'SslForm',
  props: {
    value: {
      type: Object,
      required: true
    },
    isEdit: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      formData: JSON.parse(JSON.stringify(this.value)),
      rules: {
        cert_content: [{
          required: true,
          message: this.$t('common.select_placeholder') + this.$t('page.ssl.label_cert_content'),
          type: 'error'
        }],
        key_content: [{
          required: true,
          message: this.$t('common.select_placeholder') + this.$t('page.ssl.label_key_content'),
          type: 'error'
        }]
      }
    };
  },
  watch: {
    value: {
      handler(newVal) {
        this.formData = JSON.parse(JSON.stringify(newVal));
      },
      deep: true
    }
  },
  methods: {
    onSubmit({ validateResult, firstError }) {
      if (validateResult === true) {
        this.$emit('submit', { result: this.formData });
      } else {
        this.$message.warning(firstError);
      }
    }
  }
});
</script>
