<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

# HomeWaf

面向个人和家庭自托管服务的 Web 应用防火墙。

HomeWaf 作为 HTTP/HTTPS 服务入口，为接入的自建服务检查请求、控制访问并显示运行状态。

[安装](#一键安装) · [首次登录](#首次登录) · [接入服务](#接入第一个服务) · [升级与卸载](#在线更新) · [安全测试](#持续安全检查) · [开发](#构建与运行) · [许可](#使用许可) · [第三方声明](THIRD_PARTY_NOTICES.md)

[![Release](https://img.shields.io/github/v/release/user1121114685/HomeWaf)](https://github.com/user1121114685/HomeWaf/releases/latest)
[![Build](https://github.com/user1121114685/HomeWaf/actions/workflows/ci-test.yml/badge.svg)](https://github.com/user1121114685/HomeWaf/actions/workflows/ci-test.yml)

## 适用范围

适合需要保护自托管 Web 服务的个人和家庭，例如：

- NAS 的 Web 管理界面、文件服务和相册。
- 摄像头、独立设备及智能家居的远程控制面板。
- 配合 VPN 回家使用的 Web 服务，以及其他个人网站。

HomeWaf 只检查经过其服务监听端口的 HTTP/HTTPS 流量；未接入或绕过该入口的流量不受保护。它不代替 VPN 或全协议网络防火墙，也不是面向海量公开流量的企业 WAF。

非商业用途免费。商业用途（包括公司内部部署）须事先取得书面许可，详见[使用许可](#使用许可)。

## 一键安装

安装脚本从 **本仓库最新正式 Release** 下载对应平台的安装包，校验 SHA256 后安装并注册系统服务。HomeWaf v3 仅支持全新安装。请为 v3 准备新的安装目录、数据卷及数据库，重新配置服务。v2 及更早版本的站点、账号和历史日志由管理员自行归档处理。v3 实例可在原目录重启并升级后续 v3 版本。

| 系统    | 架构                 | 默认安装目录              | 服务管理     |
| ------- | -------------------- | ------------------------- | ------------ |
| Windows | x64                  | `C:\ProgramData\HomeWaf`  | Windows 服务 |
| Linux   | x86_64、ARM64        | `/opt/homewaf`            | systemd      |
| macOS   | Intel、Apple Silicon | `/opt/homewaf`            | launchd      |
| Docker  | Linux amd64、arm64   | 容器 `/app`，数据放命名卷 | Docker       |

### Windows PowerShell

以**管理员身份**打开 PowerShell，复制执行：

```powershell
irm https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1 | iex
```

支持 Windows PowerShell 5.1 和 PowerShell 7。服务名为 `HomeWafService`。

### Linux 与 macOS

终端复制执行；需要 `curl`，安装服务时会通过 `sudo` 请求管理员权限：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh | sudo bash
```

脚本自动识别 Intel/AMD 64 位或 ARM64。没有 systemd 的 Linux 环境可使用 Docker，或下载脚本后加 `--no-service`，自行启动程序。

### Docker 一键运行

安装并启动 Docker 后，选择 **Linux 容器**模式。脚本从公开 Release 下载对应 Docker 引擎架构的镜像包，校验 SHA256，加载镜像并启动容器；无需镜像仓库账号。

Linux/macOS：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh | bash -s -- --docker
```

Windows PowerShell（Docker Desktop）：

```powershell
& ([scriptblock]::Create((irm https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1))) -Docker
```

安装后容器名为 `homewaf`，镜像标记为 `homewaf:<版本>` 和 `homewaf:latest`。配置、数据、密钥、日志及证书分别保存在命名卷中。管理端映射 `127.0.0.1:26666`，网站端口映射 80/TCP 和 443/TCP。

再次执行同一命令会安装最新正式版并重新创建容器，保留命名卷；新容器启动失败时尝试恢复原镜像。脚本只接管由它创建的容器，不覆盖已有的自定义部署。Linux 用户需有访问 Docker 的权限。

也可在 [Releases](https://github.com/user1121114685/HomeWaf/releases) 下载 `HomeWaf_Docker_Linux_*.tar.gz`，校验后通过 `docker load -i 文件名` 导入，再自行配置 `docker run`。仓库的 `docker-compose.yml` 用于从源码构建容器。

### 首次登录

**请勿将 HomeWaf 管理端口（默认 `26666`）暴露在公网。** 路由器、防火墙和云安全组只向公网开放受保护服务的监听端口，例如网站的 `80`、`443`。请从本机或可信内网登录。远程管理使用 VPN 或 SSH 隧道，避免通过公网反向代理或 CDN 发布管理入口。

打开 **http://127.0.0.1:26666**，账号为 `admin`，初始密码随机生成：

| 安装方式    | 查看初始密码                                                   |
| ----------- | -------------------------------------------------------------- |
| Windows     | `Get-Content C:\ProgramData\HomeWaf\data\initial_password.txt` |
| Linux/macOS | `sudo cat /opt/homewaf/data/initial_password.txt`              |
| Docker      | `docker exec homewaf cat /app/data/initial_password.txt`       |

首次启动需要初始化资源，若文件尚未生成请稍等片刻。登录后修改密码。远程服务器可以通过 SSH 隧道访问管理端，例如 `ssh -L 26666:127.0.0.1:26666 user@server`。

`initial_password.txt` 只记录首次生成的密码，后续修改或重置密码不会更新该文件。开发模式使用独立账号数据，查看密码及重置方法见[开发实例的账号与密码](#开发实例的账号与密码)。

### 接入第一个服务

1. 在控制台打开“我的服务”，选择“接入服务”，选定场景并填写现有业务的上游地址。
2. 检查配置并保存；如果服务需要访问账号或服务令牌，先完成设置，再检查并启用监听。可先选择观察模式，确认真实请求和命中结果后再决定是否阻断。
3. 将服务域名解析到 HomeWaf 所在地址，并将路由器端口转发或现有入口指向该服务的监听端口。启用监听不会自动修改这些网络设置。
4. 通过新入口发起请求，在该服务的访问记录中确认请求已经过 HomeWaf。上游原地址可能仍可直接访问；如果存在绕过入口的路径，应在网络侧限制其访问。

细节见控制台“使用指南 → 接入与验证”。

### 忘记密码与重置密码

能够登录时，在管理页面右上角账号菜单选择“修改密码”。忘记密码时，可在目标实例所在主机使用交互式维护命令 `resetpwd`，无需输入旧密码。

**原生安装：**先停止目标实例，在它的安装目录执行重置，成功后再启动。Windows 系统服务安装的示例（管理员 PowerShell）：

```powershell
Set-Location 'C:\ProgramData\HomeWaf'
.\HomeWaf64.exe stop
.\HomeWaf64.exe resetpwd
.\HomeWaf64.exe start
```

Linux x86_64 系统服务安装的示例：

```sh
cd /opt/homewaf
sudo ./HomeWafLinux64 stop
sudo ./HomeWafLinux64 resetpwd
sudo ./HomeWafLinux64 start
```

Linux ARM64 将文件名替换为 `HomeWafLinuxArm64`；macOS 按架构使用 `HomeWafDarwin64` 或 `HomeWafDarwinArm64`。自定义安装目录或改过可执行文件名时，使用对应实例的实际路径和文件名。未安装为系统服务的实例，在原终端按 `Ctrl+C` 停止，执行 `resetpwd` 后按原来的方式启动，无需使用 `stop` / `start` 服务命令。

**Docker：**保持目标容器运行，进入容器执行交互式重置，成功后重启容器：

```sh
docker exec -it homewaf /app/homewaf resetpwd
docker restart homewaf
```

容器名不是 `homewaf` 时替换为实际名称。

各安装方式的交互步骤相同：

1. 程序列出当前实例的账号，输入账号序号；存在 `admin` 时直接回车默认选择它。
2. 看到 `Reset password ... successfully` 后，保存终端紧接着显示的新随机临时密码。命令自动生成密码，不会要求输入自定义新密码，也不会将它写回 `initial_password.txt`。
3. 启动或重启实例，用所选账号和临时密码登录；页面会要求立即改成自己的新密码。

重置会修改所选账号的密码并清除其数据库中保存的登录令牌，保留站点配置和其他业务数据。请对准需要恢复的实例操作；若出现 `No accounts found in database.`，先检查程序路径、数据目录和数据库配置是否属于正确实例。

### 手动下载与自定义目录

所有二进制、安装脚本和校验清单都在 [GitHub Releases](https://github.com/user1121114685/HomeWaf/releases)。Windows 解压后可直接运行 `HomeWaf64.exe`；Linux/macOS 解压后运行对应的 `HomeWafLinux64`、`HomeWafLinuxArm64`、`HomeWafDarwin64` 或 `HomeWafDarwinArm64`。

自定义安装目录可使用 Shell 参数 `--dir /srv/homewaf`，或 PowerShell 参数 `-Directory 'D:\HomeWaf'`。不加 `--no-service` / `-NoService` 时，安装脚本会注册并启动系统服务。升级和卸载命令见[在线更新](#在线更新)。

## 在线更新

HomeWaf 在每个 48 小时周期内随机选一个时刻，自动检查本仓库的正式 Release。实例会保存下次检查时间，重启后继续原计划。发现新版本时，在线管理员会收到提示，离线管理员下次登录时也能看到。要通过邮件或 Webhook 接收提醒，可在「消息通知 → 事件订阅」中为相应渠道启用「新版本发布」。顶部账户菜单的「检查更新」可立即查询，安装更新需另行确认。

Windows/Linux/macOS 原生安装可在提示中确认升级。程序下载当前系统和架构的安装包，核对 GitHub 提供的 SHA256 后替换二进制，再由 Supervisor 启动新 Worker 并排空旧 Worker。更新失败、网络错误或下载校验失败会明确报错。更新渠道只指向 `user1121114685/HomeWaf`，不访问原项目的软件更新地址。

v3 安装脚本会在停止服务或替换程序前检查安装格式。v2 及更早版本必须使用新的安装目录、数据卷和数据库，旧数据由管理员自行归档。已经是 v3 的实例可通过本机认证 API 或安装脚本下载并校验包，再由 Supervisor 平滑重启。使用 `--version vX.Y.Z` / `-Version vX.Y.Z` 可检查预期版本。

新版二进制也支持 `homewaf apikey init` 后执行 `homewaf update --check`、`homewaf update --wait 15m`、`homewaf update --status`（Windows 使用 `HomeWaf64.exe`，其他平台使用对应文件名）。请求结果不确定时先查状态，避免重复提交。

Docker 更新会在加载新镜像后、停止旧容器前检查数据卷格式。v2 数据卷不能原地升级，管理员需准备新数据卷。v3 容器会下载并校验镜像、优雅停止后再重建，单容器重建期间会短暂中断服务。

### 安装脚本升级与卸载命令

每条命令都可单独运行，会下载并执行当前正式版安装脚本。以下原生服务命令适用于默认目录：Linux/macOS 为 `/opt/homewaf`，Windows 为 `C:\ProgramData\HomeWaf`；自定义目录时替换对应路径。请在可写目录运行。原生 Shell 命令使用 `sudo`，Windows 请在管理员 PowerShell 中运行。Docker 命令沿用当前用户的 Docker context；只有该 context 要求时才使用 `sudo`。

Linux/macOS 原生服务：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh -o install_homewaf.sh && sudo bash install_homewaf.sh upgrade --dir /opt/homewaf
```

卸载程序和服务，保留配置、数据库、证书及密钥：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh -o install_homewaf.sh && sudo bash install_homewaf.sh uninstall --dir /opt/homewaf
```

已备份并确认删除该实例数据时：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh -o install_homewaf.sh && sudo bash install_homewaf.sh uninstall --dir /opt/homewaf --purge-data --yes
```

Windows 原生服务：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; & ([scriptblock]::Create((Invoke-RestMethod 'https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1'))) -Action upgrade -Directory 'C:\ProgramData\HomeWaf'
```

卸载程序和服务，保留配置、数据库、证书及密钥：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; & ([scriptblock]::Create((Invoke-RestMethod 'https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1'))) -Action uninstall -Directory 'C:\ProgramData\HomeWaf'
```

已备份并确认删除该实例数据时：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; & ([scriptblock]::Create((Invoke-RestMethod 'https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1'))) -Action uninstall -Directory 'C:\ProgramData\HomeWaf' -PurgeData -Yes
```

纯前台运行的原生实例请先停止进程，然后在卸载命令中增加 `--no-service` 或 `-NoService`。`--purge-data --yes` / `-PurgeData -Yes` 只删除该实例由安装脚本管理的数据目录；容器模式下只删除 HomeWaf 安装器创建的命名卷。清理前请备份 `conf/`、`data/`、`ssl/` 和 **`.homewaf-keys/`**；数据密钥与数据库必须配套。

Docker 更新和卸载命令：

Linux/macOS 升级容器：

```sh
# Linux/macOS
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh -o install_homewaf.sh && bash install_homewaf.sh upgrade --docker
```

Linux/macOS 卸载容器并保留数据卷：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh -o install_homewaf.sh && bash install_homewaf.sh uninstall --docker
```

已备份并确认删除 Docker 数据卷时：

```sh
curl -fsSL https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.sh -o install_homewaf.sh && bash install_homewaf.sh uninstall --docker --purge-data --yes
```

Windows PowerShell / Docker Desktop 升级容器：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; & ([scriptblock]::Create((Invoke-RestMethod 'https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1'))) -Action upgrade -Docker
```

Windows PowerShell / Docker Desktop 卸载容器并保留数据卷：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; & ([scriptblock]::Create((Invoke-RestMethod 'https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1'))) -Action uninstall -Docker
```

已备份并确认删除 Docker 数据卷时：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; & ([scriptblock]::Create((Invoke-RestMethod 'https://github.com/user1121114685/HomeWaf/releases/latest/download/install_homewaf.ps1'))) -Action uninstall -Docker -PurgeData -Yes
```

### 只重启当前版本

如果只需重启 Worker、不下载新版本，可在实例目录运行 `rolling-restart`：

```powershell
.\HomeWaf64.exe rolling-restart
```

```sh
sudo ./HomeWafLinux64 rolling-restart
```

其他架构请使用对应的二进制名称，例如 `HomeWafLinuxArm64`、`HomeWafDarwin64` 或 `HomeWafDarwinArm64`。Supervisor 会先等待新 Worker 就绪，再排空旧 Worker；启动失败时旧 Worker 继续服务。默认排空时间为 30 秒，可在 `conf/config.yml` 中通过 `drain_time_out` 修改。Windows 接管监听端口期间，新连接可能需要重试；超出排空期限的长连接会关闭。

## 构建与运行

开发者可以从[项目架构](docs/ARCHITECTURE.md)查找管理接口、端口配置和防护运行时的代码入口。

### 环境要求

| 工具     | 仓库使用的版本或要求             |
| -------- | -------------------------------- |
| Go       | `1.27.4`，见 `go.mod`            |
| Node.js  | `26.13.0`，见 `web/package.json` |
| C 编译器 | 不需要；默认 `CGO_ENABLED=0`     |

Go 入口为 `cmd/homewaf`，前端源码和依赖锁文件位于 `web/`。

数据库采用 SQLite，敏感字段使用实例密钥加密。部署和备份时请单独保留 `.homewaf-keys/`；安全边界见[安全与核心设计边界](#安全与核心设计边界)。

### 从源码构建

在仓库根目录执行：

```sh
node scripts/build.mjs
node scripts/verify.mjs
```

构建脚本按锁文件安装依赖，执行 Vue/TypeScript 检查，构建前端并编译 Go 程序。产物位于 `artifacts/`，第三方许可声明会随前端一起嵌入。

前端开发：

```sh
npm run dev --prefix web
```

后端地址通过 `web/.env` 中的 `HOMEWAF_BACKEND` 指定。构建和验证入口分别为 `scripts/build.mjs`、`scripts/verify.mjs`。

### 一键开发调试与热重载

Windows 双击仓库根目录的 **`dev.bat`**，或在 PowerShell 执行：

```powershell
.\dev.bat
```

Linux/macOS 执行 `sh dev.sh`。三种系统也都可以使用 `node scripts/dev.mjs`。环境要求与源码构建相同；使用 Node 自带的文件监听，无需安装 Air 或 GoFrame。

脚本同时启动后端和 Vite 前端；Vite 入口文件缺失时（包括安装中断后仅残留目录的情况），自动执行 `npm ci --include=dev --prefix web`，并在启动后端前验证 Vite 能否运行。依赖检查失败时按提示重新安装后重试。启动完成后访问 **http://127.0.0.1:3001**。仅需后端时执行 `dev.bat --backend-only`、`sh dev.sh --backend-only` 或 `node scripts/dev.mjs --backend-only`。

保存 Go 文件后，脚本会编译候选程序并请求 Supervisor 交接 Worker。编译失败或新 Worker 未就绪时，原服务继续运行；修正代码并再次保存即可重试。修改 Supervisor、IPC 协议或开发脚本后，请停止并重新启动开发环境。

- 配置、数据库、密钥和日志保存在 `.cache/dev/runtime/`，与已安装实例隔离；开发账号的初始密码保存在该目录的 `data/initial_password.txt`。查看及重置步骤见[开发实例的账号与密码](#开发实例的账号与密码)。
- 前端入口为 `http://127.0.0.1:3001`，后端默认端口为 `26666`。可在 `.cache/dev/runtime/conf/config.yml` 中修改 `local_port`；前端代理会使用该端口。
- Go 源码、模块文件和内嵌资源变化会触发编译；单独修改 `_test.go` 不会触发编译。前端改动使用 Vite 热更新。按 `Ctrl+C` 停止本脚本启动的进程，不会影响已安装服务。

开发热重启不会保留进程内存、调用栈或 IDE 断点。

#### 开发实例的账号与密码

`dev.bat` / `dev.sh` 使用 `.cache/dev/runtime/` 内的独立配置和账号数据库，不会自动沿用仓库根目录或正式安装实例的密码。默认账号为 `admin`；在仓库根目录查看开发实例的初始密码：

```powershell
Get-Content .\.cache\dev\runtime\data\initial_password.txt -Encoding UTF8
```

Linux/macOS 使用 `cat .cache/dev/runtime/data/initial_password.txt`。开发页面入口是 **http://127.0.0.1:3001**；`26666` 是后端 API 端口，开发模式下不提供管理首页。

如果初始密码已失效或忘记当前密码，先在开发终端按 `Ctrl+C` 并等待进程退出，再从仓库根目录执行：

```powershell
Push-Location .cache\dev\runtime
.\homewaf.exe resetpwd
Pop-Location
.\dev.bat
```

Linux/macOS 从仓库根目录执行：

```sh
(cd .cache/dev/runtime && ./homewaf resetpwd)
sh dev.sh
```

按提示选择账号（回车默认选择 `admin`），保存命令显示的新随机临时密码，确认重置成功后再启动开发脚本。到 `3001` 登录并完成强制改密；账号选择和重置行为与上方[忘记密码与重置密码](#忘记密码与重置密码)一致。

### 使用 Docker Compose

```sh
docker compose build
docker compose up -d
```

默认管理端仅映射到宿主机 `127.0.0.1:26666`。远程访问方法见[首次登录](#首次登录)。

数据保存在 Compose 配置映射的 `conf/`、`data/`、`ssl/` 和 `logs/` 目录。升级前请备份；不要让两个独立实例同时使用相同端口或数据目录。

## 使用许可

> **仅非商业用途免费。商业用途（包括公司内部部署）必须事先获得书面商业授权。**

HomeWaf 自有新增代码及修改使用根目录 [LICENSE](LICENSE) 中的源码可用许可；这不是 OSI 定义的开源许可。SamWaf、SamWafWeb 与其他第三方组件继续适用各自原有许可。

软件按现状提供，不作无故障或损失赔付承诺。完整的授权范围、免责声明与责任限制以 [LICENSE](LICENSE) 为准。

- [来源声明](NOTICE)：上游项目及修改说明。
- [第三方清单](THIRD_PARTY_NOTICES.md)：组件、版本与许可。
- [第三方许可全文](LICENSES/THIRD_PARTY.txt)：按来源索引合并保存的版权声明和完整许可文本。

## 授权与隐私

管理后台「系统设置 → HomeWaf 授权」支持查看实例 ID、验证签名文件和确认导入授权。发行方应离线保管 Ed25519 私钥，仅将可信公钥写入构建；默认空信任库不接受商业授权或插件签名。

软件更新默认检查 `user1121114685/HomeWaf` 仓库的正式 GitHub Release。上游项目的更新、公告、公网 IP 查询和数据镜像默认禁用。发行方可显式配置自己的 HTTPS 服务；主动启用证书申请、DNS、通知或其他外部集成时，会向对应服务商发送实现功能所需的数据。

更新请求默认仅携带软件版本；`update_env_report` 默认关闭，启用后增加系统、架构和运行形态，不增加永久机器码。

## 安全与核心设计边界

### 管理访问

首次登录后修改密码，远程管理配置 HTTPS。控制台管理员、管理 API Key 和受保护服务的访客是不同身份，权限由服务端校验。管理端网络访问范围见[首次登录](#首次登录)。

### 流量与防护状态

只有明确配置的可信代理可以提供真实来源 IP；不接受 `/0` 信任范围或来访者自报的转发头。调整代理链后，应核对日志、地域限制和封禁使用的来源地址。处置和防护作用于接入的监听端口，处置模式互斥；观察信号不等于阻断，保存配置不等于监听已启动，页面预览也不证明真实流量已触发。蜜罐内容由实例共享，仅仿真和记录，不执行请求者提交的命令、模板或脚本，也不向外回连。

### 证书与外部任务

HTTP-01 只允许匹配的本地挑战通过；DNS-01 需要出站 CA/DNS 访问。签发证书不代表已绑定服务或建立公网访问。手动接管证书前应停用原自动续期策略，避免两种来源互相覆盖。保存策略、任务受理、签发、部署和通知送达是不同结果；通知沿用用户设置的订阅、过滤和免打扰规则。

### 数据与问题报告

日志和导入内容均视为不可信数据；密钥、令牌和口令不得明文写入日志或浏览器持久存储。数据库、配置、证书与 `.homewaf-keys/` 应配套备份，并先在备份上验证升级；不要让两个独立实例共用数据目录。

发现漏洞请使用当前仓库启用的私密报告渠道；未启用时先联系维护者确认私密渠道。只提供必要的脱敏信息，不要在公开 Issue 中发布凭证、用户数据或未修复漏洞的利用细节。原项目的支持渠道和响应承诺不适用于 HomeWaf。

## 持续安全检查

每次 CI 和 Release、以及每日定时安全任务都会运行依赖漏洞检查、Linux/Windows 防护回归和模糊测试。防护回归使用 OWASP CRS 的固定版本及当前上游选定样本，覆盖 SQL 注入、XSS、文件包含和正常请求；这是持续回归的一部分，不是完整的 CRS 测试套件，也不能保证覆盖所有新漏洞。

## 上游来源与致谢

| 项目                         | 来源提交                                   |
| ---------------------------- | ------------------------------------------ |
| `samwafgo/SamWaf`（后端）    | `8d60305a6f5392609f39a397c26ee67e758081f7` |
| `samwafgo/SamWafWeb`（前端） | `c92ee05f84e58f27fa84beacad28071f0ba5931b` |

感谢上游作者及所有贡献者。HomeWaf 保留作者版权、来源和 Apache-2.0 文本，是独立维护的修改发行版，不代表原项目，也不使用其客服、公众号或发布渠道。
