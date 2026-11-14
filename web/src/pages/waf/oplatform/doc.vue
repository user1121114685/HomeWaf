<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="doc-container">
    <section class="api-guide">
      <h2>接入管理 API</h2><p>管理 API 用于管理此 HomeWaf 实例。它与访客访问服务时使用的服务令牌不同。</p>
      <ol><li><h3>创建调用凭据</h3><p>在 API 凭据页检查开放状态，创建凭据并设置来源限制、有效期和速率限制。</p><RouterLink to="/oplatform/key">管理 API 凭据 →</RouterLink></li>
        <li><h3>按凭据说明配置调用方</h3><p>妥善保存密钥，使用当前实例支持的认证方式和接口参数。凭据轮换后，及时更新调用方。</p><RouterLink to="/oplatform/key">查看凭据与接入说明 →</RouterLink></li>
        <li><h3>核对调用结果</h3><p>在调用记录中按凭据、路径和来源查找请求，核对响应状态与失败原因。</p><RouterLink to="/oplatform/log">查看 API 调用记录 →</RouterLink></li></ol>
      <div class="api-guide-note"><strong>定位权限或认证问题</strong><p>先检查凭据状态、有效期和来源限制。管理操作仍受服务端权限校验，页面入口可见不代表调用已获授权。</p><RouterLink to="/help">使用与部署说明 →</RouterLink></div>
    </section>
  </div>
</template>

<script>
export default {
  name: 'OPlatformDoc',
}
</script>

<style scoped>
.doc-container {
  max-width: 960px;
}
.api-guide { background: var(--hw-panel); border: 1px solid var(--hw-line); border-radius: 8px; padding: 28px; }
.api-guide h2 { font-size: 18px; margin: 0 0 8px; }
.api-guide h3 { font-size: 15px; margin: 0 0 8px; }
.api-guide p { color: var(--hw-muted); font-size: 14px; line-height: 1.8; margin: 0 0 12px; }
.api-guide ol { padding-left: 24px; margin: 24px 0; }
.api-guide li { padding: 8px 0 24px 8px; }
.api-guide a { display: inline-flex; min-height: 36px; align-items: center; color: var(--hw-accent); }
.api-guide-note { border-top: 1px solid var(--hw-line); padding-top: 20px; }
.api-guide-note strong { display: block; margin-bottom: 8px; }
@media(max-width:600px) { .api-guide { padding:20px; } }
</style>
