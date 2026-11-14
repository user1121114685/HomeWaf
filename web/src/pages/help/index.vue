<template>
  <div class="help-page">
    <header class="help-header">
      <RouterLink to="/dashboard/start" class="help-brand">HomeWaf</RouterLink
      ><RouterLink to="/dashboard/start">返回控制台 ↗</RouterLink>
    </header>
    <main class="help-layout">
      <aside class="help-navigation">
        <span>使用指南</span><a href="#installation" @click.prevent="jump('installation')">安装与升级</a
        ><a href="#license" @click.prevent="jump('license')">许可边界</a
        ><a href="#commercial" @click.prevent="jump('commercial')">商业授权</a
        ><a href="#privacy" @click.prevent="jump('privacy')">隐私与更新</a>
      </aside>
      <article class="help-document">
        <div class="page-eyebrow">文档</div>
        <h1>使用与许可说明</h1>
        <p>HomeWaf 是基于 SamWaf 的独立修改发行版，与原项目不存在官方支持或授权背书关系。</p>
        <h2 id="license">许可边界</h2>
        <p>
          HomeWaf
          自有新增代码及修改采用非商业源码可用许可。非商业用途免费；公司内部部署、业务防护、收费托管、转售及其他商业用途均需事先取得书面商业授权。这不属于
          OSI 定义的开源许可。
        </p>
        <p>原项目和第三方组件继续适用各自许可；本产品条款不撤销其原有授权。</p>
        <p>
          <a href="/legal/LICENSE" target="_blank" rel="noopener">产品许可全文</a> ·
          <a href="/legal/NOTICE" target="_blank" rel="noopener">来源声明</a> ·
          <a href="/legal/THIRD_PARTY_NOTICES.md" target="_blank" rel="noopener">第三方清单</a> ·
          <a href="/legal/LICENSES/THIRD_PARTY.txt" target="_blank" rel="noopener">第三方许可全文</a>
        </p>
        <h2 id="installation">安装与升级</h2>
        <p>
          发布前请备份 conf、data、ssl、plugins
          目录及授权文件。沿用既有数据库与协议兼容标识；不要同时运行新旧服务。详细步骤见发行包
          docs/modernization/MIGRATION.md。
        </p>
        <p>
          管理端初始口令由本机首次启动生成。首次登录后请修改密码并限制管理端访问范围。后台 API 权限以服务端校验为准。
        </p>
        <h2 id="commercial">商业授权</h2>
        <p>
          从发行方向本实例申请签名授权文件，在授权管理中导入。源码仓库不包含生产签名私钥。许可价格、主体、有效期限与服务范围以双方书面合同为准。
        </p>
        <h2 id="privacy">隐私与更新</h2>
        <p>
          默认不向原项目服务发送更新、公告、机器识别码或公网 IP 查询。发行方可显式配置自己的 HTTPS 更新端点。主动启用
          DNS、证书申请、通知或 AI 服务会向对应服务商发送实现功能所需的数据。
        </p>
        <p>
          构建、迁移与发布步骤见源码 docs/modernization/MIGRATION.md；内嵌资源来源见同目录的
          provenance.json，依赖版本及许可见第三方清单。
        </p>
      </article>
    </main>
  </div>
</template>
<script setup lang="ts">
function jump(id: string) {
  document.getElementById(id)?.scrollIntoView({ block: 'start' });
}
</script>
<style scoped>
.help-page {
  min-height: 100vh;
  background: var(--hw-panel);
}
.help-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 24px max(24px, calc((100vw - 1200px) / 2));
  border-bottom: 1px solid var(--hw-line);
}
.help-brand {
  color: var(--hw-ink);
  font: 500 24px var(--font-display);
  letter-spacing: -0.04em;
}
.help-layout {
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
  gap: 64px;
  max-width: 1200px;
  margin: 0 auto;
  padding: 48px 24px 80px;
}
.help-navigation {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.help-navigation span {
  color: var(--hw-muted);
  font-size: 13px;
  padding: 8px 12px 16px;
}
.help-navigation a {
  color: var(--hw-muted);
  padding: 12px;
  border-radius: var(--hw-r-sm);
}
.help-navigation a:hover {
  background: var(--hw-subtle);
  color: var(--hw-accent);
}
.help-document {
  max-width: 760px;
}
h1 {
  font-size: 36px;
  line-height: 1.3;
  font-weight: 500;
  letter-spacing: -0.035em;
  margin: 12px 0 24px;
}
h2 {
  font-size: 22px;
  line-height: 1.5;
  font-weight: 500;
  margin: 40px 0 16px;
  scroll-margin-top: 24px;
}
p {
  color: var(--hw-muted);
  line-height: 1.9;
  margin-bottom: 16px;
  font-size: 15px;
}
a {
  overflow-wrap: anywhere;
}
@media (max-width: 760px) {
  .help-header {
    padding: 20px 24px;
  }
  .help-layout {
    display: block;
    padding-top: 24px;
  }
  .help-navigation {
    display: flex;
    flex-flow: row wrap;
    gap: 8px;
    margin-bottom: 32px;
  }
  .help-navigation span {
    width: 100%;
    padding: 0;
  }
  .help-navigation a {
    padding: 8px 0;
    margin-right: 16px;
    min-height: 44px;
  }
  h1 {
    font-size: 28px;
  }
}
</style>
