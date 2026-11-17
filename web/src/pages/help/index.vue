<template>
  <div class="help-page">
    <header class="help-header">
      <RouterLink to="/dashboard/base" class="help-brand">HomeWaf</RouterLink
      ><RouterLink to="/dashboard/base">返回控制台 ↗</RouterLink>
    </header>
    <main class="help-layout">
      <aside class="help-navigation">
        <span>使用指南</span><a href="#connect" @click.prevent="jump('connect')">接入与验证</a
        ><a href="#certificates" @click.prevent="jump('certificates')">证书与 HTTPS</a
        ><a href="#troubleshoot" @click.prevent="jump('troubleshoot')">访问排错</a
        ><a href="#installation" @click.prevent="jump('installation')">安装与升级</a
        ><a href="#license" @click.prevent="jump('license')">许可边界</a
        ><a href="#commercial" @click.prevent="jump('commercial')">商业授权</a
        ><a href="#privacy" @click.prevent="jump('privacy')">隐私与更新</a>
      </aside>
      <article class="help-document">
        <div class="page-eyebrow">文档</div>
        <h1>使用指南</h1>
        <p>从接入一个服务开始，再确认实际访问与防护结果。</p>
        <h2 id="connect">接入与验证</h2>
        <ol class="help-steps">
          <li><RouterLink to="/waf-pipeline/ports">打开我的服务</RouterLink>，选择场景并填写现有服务地址。</li>
          <li>检查配置后保存。按需配置访问账号或服务令牌，再启用监听。</li>
          <li>通过 HomeWaf 新入口访问一次，在该服务的访问记录中确认请求。监听启动不代表 DNS 或路由器映射已完成。</li>
        </ol>
        <h2 id="certificates">证书与 HTTPS</h2>
        <p>
          <RouterLink to="/waf-host/wafsslconfiglist">证书管理</RouterLink
          >中申请证书，并在单张证书中设置自动续期。<RouterLink to="/waf-host/wafsslOrderlist">申请与续期记录</RouterLink
          >用于查看任务进度、错误和恢复结果。家庭网络可使用 DNS 验证，按提供商要求填写凭据。签发后检查证书关联与服务
          HTTPS 配置。
        </p>
        <p>
          <RouterLink to="/waf-host/wafsslconfiglist">证书管理</RouterLink
          >用于导入、关联服务、维护更新策略和处理失败；<RouterLink to="/waf-host/wafsslExpirelist"
            >外部证书监测</RouterLink
          >用于检查域名和端口实际提供的证书，不是自动续期的开关。
        </p>
        <h2 id="troubleshoot">访问排错</h2>
        <ol class="help-steps">
          <li>在<RouterLink to="/waf/wafvisitlog">访问日志</RouterLink>选择服务和时间，先确认是否收到请求。</li>
          <li>打开请求详情，区分已拦截、观察命中与 HTTP 响应错误，再调整对应服务策略。</li>
          <li>
            没有请求或访问仍失败时，检查入口、监听、证书与上游；使用<RouterLink to="/sys/RunDiagnostic"
              >运行诊断</RouterLink
            >查看实例状态。
          </li>
        </ol>
        <h2>发行与许可</h2>
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
          升级前请配套备份 conf、data、ssl、plugins、.homewaf-keys
          目录及授权文件。不要让两个独立实例共用数据目录；内置平滑升级由 Supervisor 管理交接。安装与更新步骤见发行包 README.md。
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
          DNS、证书申请或通知服务会向对应服务商发送实现功能所需的数据。
        </p>
        <p>
          构建与安全边界见源码 README.md；上游来源见来源声明，内嵌资源、依赖版本及许可见第三方清单。
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
.help-steps {
  padding-left: 22px;
  color: var(--hw-muted);
  line-height: 1.9;
}
.help-steps li {
  padding: 4px 0 8px 4px;
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
