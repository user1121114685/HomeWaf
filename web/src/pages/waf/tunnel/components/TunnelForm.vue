<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
    <div>
        <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="150">

            <hw-form-item :label="$t('page.tunnel.name')" name="name">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.name"></hw-input>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.port')" name="port">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.port"></hw-input>
            </hw-form-item>

            <div class="form-row">
                <hw-form-item :label="$t('page.tunnel.protocol')" name="protocol" class="half-width" >
                    <hw-select v-model="formData.protocol" clearable :style="{ width: 'min(100%, 480px)' }" :disabled="isEdit">
                        <hw-option value="tcp" label="TCP">TCP</hw-option>
                      <hw-option value="udp" label="UDP">UDP</hw-option>
                    </hw-select>
                </hw-form-item>

                <hw-form-item :label="$t('page.tunnel.start_status')" name="start_status" class="half-width">
                    <hw-radio-group v-model="formData.start_status">
                        <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
                        <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
                    </hw-radio-group>
                </hw-form-item>
            </div>
            <hw-form-item :label="$t('page.tunnel.remote_port')" name="remote_port">
                <hw-input-number :style="{ width: '150px' }" v-model="formData.remote_port" :min="1" :max="65535"
                    :placeholder="$t('common.placeholder')">
                </hw-input-number>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.remote_ip')" name="remote_ip">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.remote_ip"></hw-input>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.allow_ip')" name="allow_ip">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.allow_ip"></hw-input>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.deny_ip')" name="deny_ip">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.deny_ip"></hw-input>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.allowed_time_ranges')" name="allowed_time_ranges">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.allowed_time_ranges"
                    :placeholder="$t('page.tunnel.allowed_time_ranges_placeholder')">
                </hw-input>
                <div style="color: #909399; font-size: 12px; margin-top: 4px;">
                    {{ $t('page.tunnel.allowed_time_ranges_tips') }}
                </div>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.ip_version')" name="ip_version">
                <hw-select v-model="formData.ip_version" :style="{ width: 'min(100%, 480px)' }">
                    <hw-option value="ipv4" :label="$t('page.tunnel.ip_version_ipv4')">IPv4</hw-option>
                    <hw-option value="ipv6" :label="$t('page.tunnel.ip_version_ipv6')">IPv6</hw-option>
                    <hw-option value="both" :label="$t('page.tunnel.ip_version_both')">IPv4 & IPv6</hw-option>
                </hw-select>
                <div style="color: #909399; font-size: 12px; margin-top: 4px;">
                    {{ $t('page.tunnel.ip_version_tips') }}
                </div>
            </hw-form-item>

            <hw-form-item v-if="formData.protocol === 'tcp'" :label="$t('page.tunnel.ssl_status')" name="ssl_status">
                <hw-radio-group v-model="formData.ssl_status">
                    <hw-radio :value="0">{{ $t('common.off') }}</hw-radio>
                    <hw-radio :value="1">{{ $t('common.on') }}</hw-radio>
                </hw-radio-group>
                <div style="color: #909399; font-size: 12px; margin-top: 4px;">
                    {{ $t('page.tunnel.ssl_tips') }}
                </div>
            </hw-form-item>

            <template v-if="formData.protocol === 'tcp' && formData.ssl_status === 1">
                <hw-form-item :label="$t('page.tunnel.ssl_certificate')" name="ssl_certificate">
                    <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.ssl_certificate"
                        placeholder="/home/nginx/cert/test.com.pem">
                    </hw-input>
                </hw-form-item>

                <hw-form-item :label="$t('page.tunnel.ssl_certificate_key')" name="ssl_certificate_key">
                    <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.ssl_certificate_key"
                        placeholder="/home/nginx/cert/test.com.key">
                    </hw-input>
                </hw-form-item>

                <hw-form-item :label="$t('page.tunnel.ssl_protocols')" name="ssl_protocols">
                    <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.ssl_protocols"
                        :placeholder="$t('page.tunnel.ssl_protocols_placeholder')">
                    </hw-input>
                </hw-form-item>
            </template>

            <div class="form-row">

                <hw-form-item :label="$t('page.tunnel.conn_timeout')" name="conn_timeout" class="half-width">
                    <hw-input-number :style="{ width: '150px' }" v-model="formData.conn_timeout"  :min="0"
                        :placeholder="$t('common.placeholder')">
                    </hw-input-number>
                </hw-form-item>

                <hw-form-item :label="$t('page.tunnel.read_timeout')" name="read_timeout" class="half-width">
                    <hw-input-number :style="{ width: '150px' }" v-model="formData.read_timeout"  :min="0"
                        :placeholder="$t('common.placeholder')">
                    </hw-input-number>
                </hw-form-item>
            </div>

            <div class="form-row">
                <hw-form-item :label="$t('page.tunnel.write_timeout')" name="write_timeout" class="half-width">
                    <hw-input-number :style="{ width: '150px' }" v-model="formData.write_timeout"  :min="0"
                        :placeholder="$t('common.placeholder')">
                    </hw-input-number>
                </hw-form-item>

                <hw-form-item :label="$t('page.tunnel.max_in_connect')" name="max_in_connect" class="half-width">
                    <hw-input-number :style="{ width: '150px' }" v-model="formData.max_in_connect"  :min="0"
                        :placeholder="$t('common.placeholder')">
                    </hw-input-number>
                </hw-form-item>
            </div>

            <hw-form-item :label="$t('page.tunnel.max_out_connect')" name="max_out_connect">
                <hw-input-number :style="{ width: '150px' }" v-model="formData.max_out_connect"  :min="0"
                    :placeholder="$t('common.placeholder')">
                </hw-input-number>
            </hw-form-item>

            <hw-form-item :label="$t('page.tunnel.remark')" name="remark">
                <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.remark"></hw-input>
            </hw-form-item>

            <hw-form-item style="float: right">
                <hw-button variant="outline" @click="$emit('close')">{{ $t('common.close') }}</hw-button>
                <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
            </hw-form-item>
        </hw-form>
    </div>
</template>

<script>
export default {
    name: 'TunnelForm',
    props: {
        value: {
            type: Object,
            default: () => ({}),
        },
        isEdit: {
            type: Boolean,
            default: false,
        },
    },
    data() {
        return {
            formData: {},
            rules: {
                name: [{ required: true, message: this.$t('common.required'), type: 'error' }],
                port: [{ required: true, message: this.$t('common.required'), type: 'error' }],
                protocol: [{ required: true, message: this.$t('common.required'), type: 'error' }],
                remote_port: [{ required: true, message: this.$t('common.required'), type: 'error' }],
                remote_ip: [{ required: true, message: this.$t('common.required'), type: 'error' }],
                allowed_time_ranges: [
                    {
                        validator: (val) => {
                            // 如果为空，允许通过（表示不限制）
                            if (!val || val.trim() === '') {
                                return { result: true };
                            }

                            // 验证时间段格式：HH:MM-HH:MM;HH:MM-HH:MM
                            const timeRangePattern = /^(\d{2}:\d{2}-\d{2}:\d{2})(;\d{2}:\d{2}-\d{2}:\d{2})*$/;
                            if (!timeRangePattern.test(val.trim())) {
                                return {
                                    result: false,
                                    message: '时间段格式错误，正确格式：08:00-10:00;11:00-12:00',
                                    type: 'error'
                                };
                            }

                            // 验证每个时间段的有效性
                            const ranges = val.trim().split(';');
                            for (let range of ranges) {
                                const times = range.split('-');
                                if (times.length !== 2) {
                                    return {
                                        result: false,
                                        message: '时间段格式错误，每个时间段应为 HH:MM-HH:MM',
                                        type: 'error'
                                    };
                                }

                                // 验证每个时间的有效性
                                for (let time of times) {
                                    const parts = time.split(':');
                                    if (parts.length !== 2) {
                                        return {
                                            result: false,
                                            message: '时间格式错误，应为 HH:MM',
                                            type: 'error'
                                        };
                                    }

                                    const hour = parseInt(parts[0]);
                                    const minute = parseInt(parts[1]);

                                    if (isNaN(hour) || hour < 0 || hour > 23) {
                                        return {
                                            result: false,
                                            message: '小时必须在 00-23 之间',
                                            type: 'error'
                                        };
                                    }

                                    if (isNaN(minute) || minute < 0 || minute > 59) {
                                        return {
                                            result: false,
                                            message: '分钟必须在 00-59 之间',
                                            type: 'error'
                                        };
                                    }
                                }
                            }

                            return { result: true };
                        },
                        type: 'error'
                    }
                ],
            },
        };
    },
    watch: {
        value: {
            handler(val) {
                this.formData = { ...val };
            },
            immediate: true,
            deep: true,
        },
    },
    methods: {
        onSubmit({ validateResult, firstError }) {
            let that = this;
            if (validateResult === true) {
                let postdata = {
                    ...that.formData
                }
                postdata['remote_port'] = Number(postdata['remote_port'])
                postdata['start_status'] = Number(postdata['start_status'])
                postdata['conn_timeout'] = Number(postdata['conn_timeout'])
                postdata['read_timeout'] = Number(postdata['read_timeout'])
                postdata['write_timeout'] = Number(postdata['write_timeout'])
                postdata['max_in_connect'] = Number(postdata['max_in_connect'])
                postdata['max_out_connect'] = Number(postdata['max_out_connect'])
                postdata['ssl_status'] = Number(postdata['ssl_status'] || 0)
                this.$emit('submit', { result: postdata });
            } else {
                console.log('Tunnel Form Error:', firstError);
                this.$message.warning(firstError);
            }
        },
    },
};
</script>

<style lang="less" scoped>
.form-row {
    display: flex;
    flex-wrap: wrap;
    margin: 0 -10px;

    .half-width {
        flex: 0 0 50%;
        max-width: 50%;
        padding: 0 10px;
        box-sizing: border-box;
    }
}

@media (max-width: 768px) {
    .form-row .half-width {
        flex: 0 0 100%;
        max-width: 100%;
    }
}
</style>
