// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { defineComponent, h, ref } from 'vue';
import { ElUpload, ElButton } from 'element-plus';

export default defineComponent({
  name: 'HwUpload',
  inheritAttrs: false,
  props: [
    'files',
    'modelValue',
    'requestMethod',
    'autoUpload',
    'action',
    'max',
    'multiple',
    'accept',
    'sizeLimit',
    'disabled',
  ],
  emits: [
    'update:files',
    'update:modelValue',
    'change',
    'success',
    'fail',
    'remove',
    'select-change',
    'progress',
    'validate',
  ],
  setup(p, { attrs, slots, emit, expose }) {
    const upload = ref<any>();
    const update = (files: any[]) => {
      emit('update:files', files);
      emit('update:modelValue', files);
    };
    expose({
      uploadFiles: () => upload.value?.submit(),
      submit: () => upload.value?.submit(),
      clearFiles: () => {
        upload.value?.clearFiles();
        update([]);
      },
      abort: (file: any) => upload.value?.abort(file),
    });
    return () =>
      h(
        ElUpload,
        {
          ...attrs,
          ref: upload,
          fileList: p.files ?? p.modelValue ?? [],
          action: p.action || '',
          multiple: p.multiple,
          accept: p.accept,
          disabled: p.disabled,
          limit: p.max,
          autoUpload: p.autoUpload !== false && !!(p.action || p.requestMethod),
          beforeUpload: async (file: File) => {
            const limit = p.sizeLimit;
            if (limit) {
              const units: Record<string, number> = { B: 1, KB: 1024, MB: 1024 ** 2, GB: 1024 ** 3 };
              const bytes =
                typeof limit === 'number'
                  ? limit
                  : Number(limit.size) * (units[String(limit.unit || 'KB').toUpperCase()] || 1);
              if (file.size > bytes) {
                emit('validate', { type: 'FILE_OVER_SIZE_LIMIT', files: [file] });
                return false;
              }
            }
            const handler = attrs.beforeUpload || attrs['before-upload'];
            return typeof handler === 'function' ? await handler(file) : true;
          },
          httpRequest: p.requestMethod
            ? async (options: any) => {
                const file = {
                  raw: options.file,
                  name: options.file.name,
                  size: options.file.size,
                  type: options.file.type,
                };
                const result = await p.requestMethod(p.multiple ? [file] : file);
                if (result?.status === 'fail') throw new Error(result.error || result.response?.msg || 'Upload failed');
                return result?.response ?? result;
              }
            : undefined,
          onChange: (file: any, files: any[]) => {
            update(files);
            emit('change', files, { file });
            if (file.status === 'ready') emit('select-change', files, { currentSelectedFiles: [file] });
          },
          onSuccess: (response: any, file: any, files: any[]) => {
            update(files);
            emit('success', { response, file, fileList: files });
          },
          onError: (error: Error, file: any, files: any[]) => emit('fail', { error, file, files }),
          onRemove: (file: any, files: any[]) => {
            update(files);
            emit('remove', { file, files });
          },
          onProgress: (event: any, file: any, files: any[]) => emit('progress', { ...event, file, files }),
        },
        {
          ...slots,
          default:
            slots.default ||
            (() =>
              h(ElButton, { type: 'primary' }, () =>
                String(attrs.triggerButton || attrs['trigger-button'] || '选择文件'),
              )),
        },
      );
  },
});
