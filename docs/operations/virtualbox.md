# GitHub neko233-com/vbox 本机验证

本项目中的 `vbox` 指本机 checkout：
`C:\tools\vbox`，对应 GitHub
仓库 `github.com/neko233-com/vbox`。它替代的是 `docker` 命令本身，是 Docker
兼容的 OCI 容器运行器，不是 Oracle VirtualBox，也不是 Vagrant；vbox 不启动或依赖
Docker Desktop/daemon。

## 前置条件

- 在 vbox 仓库中安装 Rust stable 与 Cargo；
- Windows 已启用 WSL2，vbox 会维护独立的 `vbox-runtime` distro；
- gitsss 镜像已在 registry 中，且当前 vbox 能访问它；私有 registry 需要按 vbox 文档提供凭据；
- PowerShell 5.1+。

从 vbox checkout 构建并安装：

```powershell
Set-Location 'C:\tools\vbox'
.\install.ps1
```

## gitsss 验证

在 gitsss checkout 根目录执行：

```powershell
.\tools\vbox\gitsss.ps1 -Action check
.\tools\vbox\gitsss.ps1 -Action pull
.\tools\vbox\gitsss.ps1 -Action install
.\tools\vbox\gitsss.ps1 -Action verify
```

如果 registry 不可访问但手上有 GitSSS 发布流程生成的 Docker archive，可以直接由
vbox 导入，不需要 Docker：

```powershell
.\tools\vbox\gitsss.ps1 -Action load -ImageArchive .\gitsss-image.tar
.\tools\vbox\gitsss.ps1 -Action install -ImageArchive .\gitsss-image.tar
.\tools\vbox\gitsss.ps1 -Action verify -ImageArchive .\gitsss-image.tar
```

验证 helper 使用 `vbox run`，也可以直接让原始 Compose 文件通过 vbox 工作：

- HTTP 映射到 `127.0.0.1:3000`；
- Git SSH 映射到 `127.0.0.1:2222`；
- `%LOCALAPPDATA%\gitsss\vbox-data` 以卷映射挂载到 `/data`；
- 使用 GitSSS 镜像的 `/usr/local/bin/gitsss` 入口直接启动 Web 进程；
- 通过 `/api/healthz` 等待服务就绪。

```powershell
$env:Path = "$env:LOCALAPPDATA\vbox\bin;$env:Path"
docker compose -f compose.yaml up -d
docker compose -f compose.yaml ps
docker compose -f compose.yaml down
```

Dockerfile、`compose.yaml`、镜像名和卷路径保持不变；只有 Docker 命令/API 的提供方从
Docker daemon 换为 vbox。vbox 本身不提供多机网络或 Swarm 控制面。

当前镜像仓库如果返回 401，不应把它当成 vbox 运行时成功。可在 vbox 进程环境中设置
`VBOX_REGISTRY_USERNAME`、`VBOX_REGISTRY_PASSWORD`，或者配置匹配 registry 的
`%USERPROFILE%/.docker/config.json` `auths` 项；vbox 不调用 credential helper。

`vbox` 的 `--volume HOST:CONTAINER[:ro]` 是 GitSSS 持久化数据的必要能力。它在
Windows + Linux 镜像路径中将主机目录映射到 WSL，再 bind mount 到容器 rootfs；容器
停止后卸载挂载点。旧版 vbox 若不支持 `--volume`，应先更新
`github.com/neko233-com/vbox` 的本地 checkout。

`vbox load --input` 支持 Docker `save` 生成的 `.tar`/`.tar.gz` archive，导入时会保留
镜像配置、entrypoint、command、环境变量和 layer；GitSSS 发布包可用该方式绕过 Docker
runtime，仅把 Docker archive 当作离线镜像载体。

```powershell
# 保留容器继续运行
.\tools\vbox\gitsss.ps1 -Action verify -KeepRunning

# 查看、停止、删除容器；remove 不删除持久化数据目录
.\tools\vbox\gitsss.ps1 -Action logs
.\tools\vbox\gitsss.ps1 -Action stop
.\tools\vbox\gitsss.ps1 -Action remove
```

Docker 安装器和 Compose 的文件契约仍然保留；vbox 是本机 Docker 命令替代和轻量本地
运行路径，不改变 Git 存储路径、数据卷语义或发布镜像名称。Swarm、Kubernetes、多机
网络和其他平台控制面不在本机 vbox 引擎范围内。
