<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

# HomeWaf

**首款面向家庭用户的 WAF（Web 应用防火墙），让自建服务的访问控制更简单。**

HomeWaf 是基于 SamWaf 的独立修改发行版，重点服务于家庭和个人自建服务场景，以简单、易用为目标。

[一键安装](#一键安装) · [Docker](#docker-一键运行) · [在线更新](#在线更新) · [源码构建](#构建与运行) · [使用许可](#使用许可) · [第三方声明](THIRD_PARTY_NOTICES.md)

[![Release](https://img.shields.io/github/v/release/user1121114685/HomeWaf)](https://github.com/user1121114685/HomeWaf/releases/latest)
[![Build](https://github.com/user1121114685/HomeWaf/actions/workflows/ci-test.yml/badge.svg)](https://github.com/user1121114685/HomeWaf/actions/workflows/ci-test.yml)

## 为什么面向家庭用户

家庭用户通常只需要向家人、朋友或自己的设备开放服务，访问者往往只有几个人到十来个人，例如：

- NAS 的 Web 管理界面、文件服务和相册。
- 摄像头、独立设备及智能家居的远程控制面板。
- 配合 VPN 回家使用的 Web 服务，以及其他个人网站。

围绕这些场景，HomeWaf 的设计思路是 **IP 限制 + 蜜罐封锁 + 断网自保**：优先控制谁能访问，减少不必要的暴露，再处理获准访问后的异常请求。这里描述的是项目方向，具体能力以当前版本实际实现为准。

HomeWaf 聚焦 Web 应用防护，不代替 VPN 或全协议网络防火墙。它也不是以大型企业、海量公开访客为主要目标的产品。

## 为什么继续公开源码

1. **尊重来源。** HomeWaf 来自开源项目 SamWaf，保留上游作者的版权、来源和许可声明。
2. **共同发现问题。** 网络环境复杂，个人能覆盖的设备、场景和问题有限，需要使用者参与反馈和改进。
3. **一起把产品做好。** 现在的设计未必是最好的，欢迎提出更合适的方案。

源码公开不代表没有使用限制，具体范围见下方的[使用许可](#使用许可)。参与开发前，请阅读[贡献说明](CONTRIBUTING.md)。

## 免费与付费

HomeWaf 采用免费与付费并存的方式。面向家庭的非商业使用免费；需要持续支付服务器等成本的服务，计划通过付费分摊费用。

项目以收支平衡、持续维护为目标。如果产生结余，将继续投入开发，并向上游 SamWaf 捐赠。吃水不忘挖井人。

付费服务的范围和价格以实际公布的方案为准；目前仓库提供离线签名授权机制，云授权、订单系统和付费功能清单尚未定义。

## 一键安装

安装脚本从 **本仓库最新正式 Release** 下载对应平台的安装包，校验 SHA256 后安装并注册系统服务。首次启动直接创建当前数据库结构和默认配置，不导入旧版 SamWaf 数据，也不执行历史升级补丁。

| 系统 | 架构 | 默认安装目录 | 服务管理 |
| --- | --- | --- | --- |
| Windows | x64 | `C:\ProgramData\HomeWaf` | Windows 服务 |
| Linux | x86_64、ARM64 | `/opt/homewaf` | systemd |
| macOS | Intel、Apple Silicon | `/opt/homewaf` | launchd |
| Docker | Linux amd64、arm64 | 容器 `/app`，数据放命名卷 | Docker |

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

安装后容器名为 `homewaf`，镜像标记为 `homewaf:v2.0.1` 和 `homewaf:latest`，配置、数据、密钥、日志及证书分别保存在命名卷中。管理端映射 `127.0.0.1:26666`，网站端口映射 80/TCP、443/TCP 和 443/UDP；请确保这些端口可用。

再次执行同一命令会安装最新正式版并重新创建容器，保留命名卷；新容器启动失败时尝试恢复原镜像。脚本只接管由它创建的容器，不覆盖已有的自定义部署。Linux 用户需有访问 Docker 的权限。

也可在 [Releases](https://github.com/user1121114685/HomeWaf/releases) 下载 `HomeWaf_Docker_Linux_*.tar.gz`，校验后通过 `docker load -i 文件名` 导入，再自行配置 `docker run`。仓库的 `docker-compose.yml` 用于从源码构建容器。

### 首次登录

打开 **http://127.0.0.1:26666**，账号为 `admin`，初始密码随机生成：

| 安装方式 | 查看初始密码 |
| --- | --- |
| Windows | `Get-Content C:\ProgramData\HomeWaf\data\initial_password.txt` |
| Linux/macOS | `sudo cat /opt/homewaf/data/initial_password.txt` |
| Docker | `docker exec homewaf cat /app/data/initial_password.txt` |

首次启动需要初始化资源，若文件尚未生成请稍等片刻。登录后修改密码。远程服务器可以通过 SSH 隧道访问管理端，例如 `ssh -L 26666:127.0.0.1:26666 user@server`。

### 手动下载、自定义目录与卸载

所有二进制、安装脚本和校验清单都在 [GitHub Releases](https://github.com/user1121114685/HomeWaf/releases)。Windows 解压后可直接运行 `HomeWaf64.exe`；Linux/macOS 解压后运行对应的 `HomeWafLinux64`、`HomeWafLinuxArm64`、`HomeWafDarwin64` 或 `HomeWafDarwinArm64`。

下载脚本到本机后，可以指定 `--version v2.0.1 --dir /opt/homewaf --no-service`（Shell），或 `-Version v2.0.1 -Directory C:\HomeWaf -NoService`（PowerShell）。不加 `--no-service`/`-NoService` 时自动安装并启动服务。

卸载服务：Shell 使用 `sudo bash install_homewaf.sh uninstall`；管理员 PowerShell 使用 `./install_homewaf.ps1 -Action uninstall`。自定义目录需同时指定。卸载服务保留配置、数据库、证书和密钥。

## 在线更新

登录管理后台时会自动检查本仓库的正式 Release；发现较新版本后显示更新提示，也可以点击顶部的“检查更新”。相同版本不重复下载，测试版和草稿不会作为正式更新推送。

Windows/Linux/macOS 原生安装可在提示中确认升级。程序下载当前系统和架构的安装包，核对 GitHub 提供的 SHA256 后替换二进制，再由 Supervisor 启动新 Worker 并排空旧 Worker。更新失败、网络错误或下载校验失败会明确报错。更新渠道只指向 `user1121114685/HomeWaf`，不访问原项目的软件更新地址。

Docker 内保留版本检查和更新提示；升级时再次执行上面的 Docker 一键安装命令，下载并校验新镜像后重新创建容器。原生部署使用后台在线升级，Docker 部署使用镜像更新。

备份时保留 `conf/`、`data/`、`ssl/` 和 **`.homewaf-keys/`**；数据密钥与数据库必须配套，不能删除密钥后生成新密钥来读取原有密文。容器命名卷默认会在重建时保留，不要执行 `down -v`。

## 构建与运行

### 环境要求

| 工具     | 仓库使用的版本或要求             |
| -------- | -------------------------------- |
| Go       | `1.27.3`，见 `go.mod`            |
| Node.js  | `26.13.0`，见 `web/package.json` |
| C 编译器 | 不需要；默认 `CGO_ENABLED=0`      |

前端源码和依赖锁文件均在本仓库中。Go 主模块为 `HomeWaf`，程序入口为 `cmd/homewaf`，前端包为 `homewaf-web`。

数据库采用标准 SQLite，敏感字段使用实例密钥加密。部署和备份时请单独持久化 `.homewaf-keys/`；源码公开情况下的保护范围与授权限制见[安全说明](docs/modernization/SECURITY.md)。

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

后端地址通过 `web/.env` 中的 `HOMEWAF_BACKEND` 指定。更多构建选项、验证方式和签名工具用法见[构建与发布说明](docs/modernization/MIGRATION.md)。

### 一键开发调试与热重载

Windows 双击仓库根目录的 **`dev.bat`**，或在 PowerShell 执行：

```powershell
.\dev.bat
```

Linux/macOS 执行 `sh dev.sh`。三种系统也都可以使用 `node scripts/dev.mjs`。环境要求与源码构建相同；使用 Node 自带的文件监听，无需安装 Air 或 GoFrame。

脚本同时启动后端和 Vite 前端，缺少前端依赖时自动执行 `npm ci --prefix web`。启动完成后访问 **http://127.0.0.1:3001**。仅需后端时执行 `dev.bat --backend-only`、`sh dev.sh --backend-only` 或 `node scripts/dev.mjs --backend-only`。

保存 Go 文件后自动执行：

1. 等待 600ms 合并连续保存，编译 `./cmd/homewaf` 到独立候选文件。
2. 编译成功后替换开发实例的二进制，调用已有 `rolling-restart` 命令。
3. Supervisor 启动新 Worker，确认 READY，再排空旧 Worker 并接管独占资源。
4. 等待本轮交接完成后才处理下一轮修改，避免连续保存的重启请求被忽略。

编译错误会显示在终端，当前服务继续运行；修正后保存即可重试。新 Worker 就绪失败时保留旧 Worker，并恢复旧二进制启动路径。Windows 用唯一备份文件名移开运行中的映像，避免连续重编译时覆盖被占用的 `.exe`。

- 开发配置、数据库、密钥和日志：`.cache/dev/runtime/`；退出和重启时保留，与仓库根目录和已安装实例分开。
- 初始密码：`.cache/dev/runtime/data/initial_password.txt`；首次初始化后生成。
- 后端默认端口：`26666`。如被占用，可预先创建或修改 `.cache/dev/runtime/conf/config.yml` 中的 `local_port`；一键启动的前端代理自动使用该端口。前端固定使用 `3001`，占用时明确报错。
- 监听 Go 文件的修改、新增、删除和重命名，以及 `go.mod`、`go.sum`、Go workspace 文件和已有内嵌资源目录；仅修改 `_test.go` 不触发编译。前端改动交给 Vite 热更新。
- 停止：在启动终端按 `Ctrl+C`，脚本清理自己启动的前后端进程；不会操作已安装的系统服务。

Worker 业务代码自动生效；**修改常驻 Supervisor、IPC 协议或开发脚本本身后，请停止脚本并重新启动**。开发期平滑重启沿用下节的连接排空期限与 Windows 端口接管限制。它不保留进程内存或调用栈，也不提供 Delve/IDE 断点会话自动重连；断点调试需另行通过 IDE 配置。

### 手动平滑重启

HomeWaf 已在运行时，可在**当前实例的安装目录**执行以下命令，平滑重启当前版本，不下载新程序。

Windows（PowerShell）：

```powershell
.\HomeWaf64.exe rolling-restart
```

Linux x86_64：

```sh
./HomeWafLinux64 rolling-restart
```

Linux arm64 使用 `./HomeWafLinuxArm64 rolling-restart`。如果可执行文件已改名，请使用实际文件名。

命令连接正在运行的 Supervisor，先启动新 Worker，确认就绪后再让旧 Worker 排空并退出；新 Worker 就绪失败时，旧 Worker 继续服务。命令返回表示请求已提交，是否交接完成请查看 Supervisor 日志。

Windows 使用 `SO_REUSEADDR` 接管监听端口，切换瞬间新建 TCP 连接可能需要重试；不能对每一次新连接握手承诺零中断。已受理请求与长连接按排空期限处理。

旧业务连接的排空期限由 `conf/config.yml` 中的 `drain_time_out` 控制，默认 30 秒；超过期限的长连接会被关闭。在线升级自动使用同一交接流程。普通 `restart` 是服务的停止／启动操作，管理端“重启 WAF 引擎”用于重载引擎配置。

协议支持范围和独占资源接管限制见[平滑升级与滚动重启说明](docs/modernization/MIGRATION.md#平滑升级与滚动重启)。

### 使用 Docker Compose

```sh
docker compose build
docker compose up -d
```

默认管理端仅映射到宿主机 `127.0.0.1:26666`，请通过安全隧道访问，或自行配置访问控制。

持久化数据位于 `conf/`、`data/`、`ssl/` 和 `logs/`。升级前请备份；不要自行同时启动两个独立实例共用端口或数据目录。内置平滑升级的新旧 Worker 重叠期由 Supervisor 统一管理。

## 使用许可

> **仅非商业用途免费。商业用途（包括公司内部部署）必须事先获得书面商业授权。**

HomeWaf 自有新增代码及修改使用根目录 [LICENSE](LICENSE) 中的源码可用许可；这不是 OSI 定义的开源许可。SamWaf、SamWafWeb 与其他第三方组件继续适用各自原有许可。

软件按现状提供，不作无故障或损失赔付承诺。完整的授权范围、免责声明与责任限制以 [LICENSE](LICENSE) 为准。

- [来源声明](NOTICE)：上游项目及修改说明。
- [第三方清单](THIRD_PARTY_NOTICES.md)：组件、版本与许可。
- [第三方许可全文](LICENSES/THIRD_PARTY.txt)：按来源索引合并保存的版权声明和完整许可文本。

## 授权与隐私

管理后台「系统设置 → HomeWaf 授权」支持查看实例 ID、验证签名文件和确认导入授权。发行方应离线保管 Ed25519 私钥，仅将可信公钥写入构建；默认空信任库不接受商业授权或插件签名。

软件更新默认检查 `user1121114685/HomeWaf` 仓库的正式 GitHub Release。原项目的更新、公告、公网 IP 查询和机器人数据镜像默认禁用。发行方可显式配置自己的 HTTPS 服务；主动启用证书申请、DNS、通知或 AI 服务时，会向对应服务商发送实现功能所需的数据。

详细配置见[初始化与发布说明](docs/modernization/MIGRATION.md)，部署和问题反馈见[安全说明](SECURITY.md)。

## 上游来源与致谢

| 项目                         | 来源提交                                   |
| ---------------------------- | ------------------------------------------ |
| `samwafgo/SamWaf`（后端）    | `8d60305a6f5392609f39a397c26ee67e758081f7` |
| `samwafgo/SamWafWeb`（前端） | `c92ee05f84e58f27fa84beacad28071f0ba5931b` |

感谢上游作者及所有贡献者。HomeWaf 保留作者版权、来源和 Apache-2.0 文本，是独立维护的修改发行版，不代表原项目，也不使用其客服、公众号或发布渠道。
