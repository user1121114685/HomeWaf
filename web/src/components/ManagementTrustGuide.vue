<template>
  <section class="management-trust" aria-label="HTTPS 访问指引" data-testid="management-trust">
    <h3>使用 HTTPS 管理入口</h3>
    <p v-if="caPem">这台实例使用本地证书。请在访问用的设备上信任根证书，再打开 HTTPS。</p>
    <p v-else>管理端已配置 HTTPS，可以使用加密连接访问。</p>
    <template v-if="caPem">
      <hw-button variant="outline" @click="downloadRoot">下载 HomeWaf 根证书</hw-button>
      <details class="management-trust-steps">
        <summary>如何导入并核对根证书</summary>
        <p>先与服务器安装输出中的 SHA-256 指纹核对一致，再导入此实例的根证书。</p>
        <code>{{ fingerprint }}</code>
        <ul>
          <li>Windows / Edge / Chrome：打开下载的证书，选择“当前用户”，手动放入“受信任的根证书颁发机构”。</li>
          <li>macOS：导入“钥匙串访问”，核对 HomeWaf Local CA 的指纹后，将 SSL 信任设为“始终信任”。</li>
          <li>Linux：将证书安装到系统信任库；Firefox 等使用独立信任库的浏览器还需在“证书颁发机构”中导入。</li>
        </ul>
        <p>导入后重新打开浏览器并验证 HTTPS。只导入根证书，不要复制或分享服务器上的 CA 私钥。</p>
      </details>
    </template>
    <a v-if="httpsUrl" class="management-https-link" :href="httpsUrl">打开 HTTPS 管理端 ↗</a>
    <p v-else class="management-trust-warning">
      当前访问名不在证书中。请先通过安装提示中的服务器 IP 登录，再配置需要的管理域名。
    </p>
  </section>
</template>

<script setup lang="ts">
const props = defineProps({
  caPem: { type: String, default: '' },
  fingerprint: { type: String, default: '' },
  httpsUrl: { type: String, default: '' },
});
function downloadRoot() {
  if (!props.caPem) return;
  const url = URL.createObjectURL(new Blob([props.caPem], { type: 'application/x-x509-ca-cert' }));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = 'homewaf-local-ca.crt';
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
</script>

<style scoped>
.management-trust {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--hw-line);
  color: var(--hw-ink);
  font-size: 13px;
  line-height: 1.7;
}
.management-trust h3 {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
}
.management-trust p {
  margin: 8px 0;
}
.management-trust code {
  display: block;
  overflow-wrap: anywhere;
  font-size: 12px;
}
.management-trust ul {
  padding-left: 20px;
}
.management-trust-steps {
  margin: 12px 0;
}
.management-trust-steps summary {
  cursor: pointer;
  min-height: 32px;
}
.management-https-link {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--hw-accent);
  font-weight: 500;
}
.management-trust-warning {
  color: var(--hw-muted);
}
</style>
