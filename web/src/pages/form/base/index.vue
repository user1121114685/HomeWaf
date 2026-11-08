<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-card>
    <hw-form
      ref="form"
      :data="formData"
      :rules="FORM_RULES"
      label-align="top"
      :label-width="100"
      @reset="onReset"
      @submit="onSubmit"
    >
      <div class="form-basic-container">
        <div class="form-basic-item">
          <div class="form-basic-container-title">合同信息</div>
          <!-- 表单内容 -->

          <!-- 合同名称,合同类型 -->
          <hw-row class="row-gap" :gutter="[16, 24]">
            <hw-col :span="6">
              <hw-form-item label="合同名称" name="name">
                <hw-input v-model="formData.name" :style="{ width: '322px' }" placeholder="请输入内容" />
              </hw-form-item>
            </hw-col>
            <hw-col :span="6">
              <hw-form-item label="合同类型" name="type">
                <hw-select
                  v-model="formData.type"
                  :style="{ width: '322px' }"
                  placeholder="请选择类型"
                  class="demo-select-base"
                  clearable
                >
                  <hw-option v-for="(item, index) in typeOptions" :key="index" :value="item.value" :label="item.label">
                    {{ item.label }}
                  </hw-option>
                </hw-select>
              </hw-form-item>
            </hw-col>

            <!-- 合同收付类型 -->
            <hw-col :span="8">
              <hw-form-item label="合同收付类型" name="payment">
                <hw-radio-group v-model="formData.payment">
                  <hw-radio value="1"> 收款 </hw-radio>
                  <hw-radio value="2"> 付款 </hw-radio>
                </hw-radio-group>
                <span class="space-item" />
                <hw-input placeholder="请输入金额" :style="{ width: '160px' }" />
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="甲方" name="partyA">
                <hw-select
                  v-model="formData.partyA"
                  :style="{ width: '322px' }"
                  class="demo-select-base"
                  placeholder="请选择类型"
                  clearable
                >
                  <hw-option v-for="(item, index) in partyAOptions" :key="index" :value="item.value" :label="item.label">
                    {{ item.label }}
                  </hw-option>
                </hw-select>
              </hw-form-item>
            </hw-col>
            <hw-col :span="6">
              <hw-form-item label="乙方" name="partyB">
                <hw-select
                  v-model="formData.partyB"
                  :style="{ width: '322px' }"
                  placeholder="请选择类型"
                  class="demo-select-base"
                  clearable
                >
                  <hw-option v-for="(item, index) in partyBOptions" :key="index" :value="item.value" :label="item.label">
                    {{ item.label }}
                  </hw-option>
                </hw-select>
              </hw-form-item>
            </hw-col>
            <hw-col :span="6">
              <hw-form-item label="合同签订日期" name="signDate">
                <hw-date-picker
                  v-model="formData.signDate"
                  :style="{ width: '322px' }"
                  theme="primary"
                  mode="date"
                  separator="/"
                />
              </hw-form-item>
            </hw-col>
            <hw-col :span="6">
              <hw-form-item label="合同生效日期" name="startDate">
                <hw-date-picker
                  v-model="formData.startDate"
                  :style="{ width: '322px' }"
                  theme="primary"
                  mode="date"
                  separator="/"
                />
              </hw-form-item>
            </hw-col>
            <hw-col :span="6">
              <hw-form-item label="合同结束日期" name="endDate">
                <hw-date-picker
                  v-model="formData.endDate"
                  :style="{ width: '322px' }"
                  theme="primary"
                  mode="date"
                  separator="/"
                />
              </hw-form-item>
            </hw-col>
            <hw-col :span="6">
              <hw-form-item label="" name="files">
                <hw-upload
                  v-model="formData.files"
                  action="https://service-bv448zsw-1257786608.gz.apigw.tencentcs.com/api/upload-demo"
                  tips="请上传pdf文件，大小在60M以内"
                  :size-limit="{ size: 60, unit: 'MB' }"
                  :format-response="formatResponse"
                  :before-upload="beforeUpload"
                  @fail="handleFail"
                >
                  <hw-button class="form-submit-upload-btn" variant="outline"> 上传合同文件 </hw-button>
                </hw-upload>
              </hw-form-item>
            </hw-col>
          </hw-row>

          <div class="form-basic-container-title form-title-gap">其它信息</div>

          <hw-form-item label="备注" name="comment">
            <hw-textarea v-model="formData.comment" :height="124" placeholder="请输入备注" />
          </hw-form-item>
          <hw-form-item label="公证人">
            <hw-avatar-group>
              <hw-avatar>D</hw-avatar>
              <hw-avatar>S</hw-avatar>
              <hw-avatar>+</hw-avatar>
            </hw-avatar-group>
          </hw-form-item>
        </div>
      </div>

      <div class="form-submit-container">
        <div class="form-submit-sub">
          <div class="form-submit-left">
            <hw-button theme="primary" class="form-submit-confirm" type="submit"> 提交 </hw-button>
            <hw-button type="reset" class="form-submit-cancel" theme="default" variant="base"> 取消 </hw-button>
          </div>
        </div>
      </div>
    </hw-form>
  </hw-card>
</template>
<script>
import { prefix } from '@/config/global';

const INITIAL_DATA = {
  name: '',
  type: '',
  partyA: '',
  partyB: '',
  signDate: '',
  startDate: '',
  endDate: '',
  payment: '1',
  amount: 0,
  comment: '',
  files: [],
};
const FORM_RULES = {
  name: [{ required: true, message: '请输入合同名称', type: 'error' }],
  type: [{ required: true, message: '请选择合同类型', type: 'error' }],
  payment: [{ required: true, message: '请选择合同收付类型', type: 'error' }],
  amount: [{ required: true, message: '请输入合同金额', type: 'error' }],
  partyA: [{ required: true, message: '请选择甲方', type: 'error' }],
  partyB: [{ required: true, message: '请选择乙方', type: 'error' }],
  signDate: [{ required: true, message: '请选择日期', type: 'error' }],
  startDate: [{ required: true, message: '请选择日期', type: 'error' }],
  endDate: [{ required: true, message: '请选择日期', type: 'error' }],
};

export default {
  name: 'FormBase',
  data() {
    return {
      prefix,
      stepSuccess: true,
      formData: { ...INITIAL_DATA },
      FORM_RULES,
      typeOptions: [
        { label: '类型A', value: '1' },
        { label: '类型B', value: '2' },
        { label: '类型C', value: '3' },
      ],
      partyAOptions: [
        { label: '公司A', value: '1' },
        { label: '公司B', value: '2' },
        { label: '公司C', value: '3' },
      ],
      partyBOptions: [
        { label: '公司A', value: '1' },
        { label: '公司B', value: '2' },
        { label: '公司C', value: '3' },
      ],
      textareaValue: '',
      rules: {
        name: [{ required: true, message: '请输入合同名称', type: 'error' }],
        type: [{ required: true, message: '请选择合同类型', type: 'error' }],
        payment: [{ required: true, message: '请选择合同收付类型', type: 'error' }],
        amount: [{ required: true, message: '请输入合同金额', type: 'error' }],
        partyA: [{ required: true, message: '请选择甲方', type: 'error' }],
        partyB: [{ required: true, message: '请选择乙方', type: 'error' }],
        signDate: [{ required: true, message: '请选择日期', type: 'error' }],
        startDate: [{ required: true, message: '请选择日期', type: 'error' }],
        endDate: [{ required: true, message: '请选择日期', type: 'error' }],
      },
    };
  },
  methods: {
    handleFail({ file }) {
      this.$message.error(`文件 ${file.name} 上传失败`);
    },
    beforeUpload(file) {
      if (!/\.(pdf)$/.test(file.name)) {
        this.$message.warning('请上传pdf文件');
        return false;
      }
      return true;
    },
    // 用于格式化接口响应值，error 会被用于上传失败的提示文字；url 表示文件/图片地址
    formatResponse(res) {
      return { ...res, error: '上传失败，请重试', url: res.url };
    },
    changeStatus() {
      this.stepSuccess = !this.stepSuccess;
    },
    onReset() {
      this.$message.warning('取消新建');
    },
    onSubmit({ validateResult }) {
      if (validateResult === true) {
        this.$message.success('新建成功');
      }
    },
  },
};
</script>
<style lang="less" scoped>
@import './index';
</style>
