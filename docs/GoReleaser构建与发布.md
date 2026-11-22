# 使用 GoReleaser 构建与发布 INIS

本文说明如何用 GoReleaser 交叉编译 INIS、把 Mellow 前端产物内嵌进二进制、打 Docker 镜像，并发布到 GitHub Release 与 GHCR。

> 相关文件：`.goreleaser.yaml`、`Dockerfile`、`scripts/prepare-frontend.mjs`、`scripts/extract-release-notes.mjs`、`.github/workflows/release.yml`
>
> 如果你只是想在本机快速编一个能跑的二进制，用 `build.bat` / `build.sh` 就够了（见文末对比）；GoReleaser 面向「一次产出全部平台 + 镜像 + Release」的正式发版。

---

## 零、先把代码和环境准备好

```bash
# 1. 克隆仓库（SSH / HTTPS 任选，仓库名是 INIS-cms）
git clone git@github.com:zhu885744/INIS-cms.git
cd INIS-cms

# 2. 后端依赖
go mod download

# 3. 前端依赖（GoReleaser 构建前会自动 npm ci，提前装好可以省掉每次的检查）
npm --prefix Mellow ci

# 4. 确认工具链版本（要求见下一节）
go version
node -v
goreleaser --version
upx --version
```

如果只是想确认环境能不能跑通，先做一次校验 + 试打包，产物落在 `dist/`，不会发布任何东西：

```bash
goreleaser check
goreleaser release --snapshot --clean
```

> 只想本地把站点跑起来看效果？`go run main.go` 后访问 <http://localhost:8642>，
> 未安装会自动跳到 `/install` 向导（详见 README 的「快速开始」）。

---

## 一、一次发布产出什么

| 产物 | 说明 |
|---|---|
| `inis_<version>_linux_amd64.tar.gz` | Linux x86_64 |
| `inis_<version>_linux_arm64.tar.gz` | Linux ARM64（鲲鹏 / 树莓派 / 云 ARM） |
| `inis_<version>_windows_amd64.zip` | Windows x86_64 |
| `inis_<version>_windows_arm64.zip` | Windows ARM64 |
| `inis_<version>_darwin_amd64.tar.gz` | macOS Intel |
| `inis_<version>_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `checksums.txt` | 全部产物的 SHA-256 |
| `ghcr.io/zhu885744/inis:<version>` | Docker 镜像（amd64 + arm64 合并 manifest） |
| `ghcr.io/zhu885744/inis:latest` | 稳定版额外打的 tag |

每个压缩包里是**单个可执行文件**（自带 Mellow 前端、安装向导、语言包）+ `LICENSE` / `README.md` / `CHANGELOG.md`。部署时只需要「二进制 + config 目录」。

---

## 二、前置条件

| 依赖 | 版本要求 | 用途 |
|---|---|---|
| Go | 1.27.4+ | 编译后端；版本由 `-ldflags -X` 注入 |
| Node.js | 20.19+ 或 22.12+ | 构建 Mellow 前端（GoReleaser 的 before hook 会调用） |
| GoReleaser | v2（需支持 `dockers_v2`，建议用最新版） | 打包、归档、发版 |
| upx | 任意较新版本 | 压缩 Linux / Windows 二进制（未安装会直接报错中止） |
| Docker + buildx | 仅在构建镜像时需要 | 本地可跳过（snapshot 默认不构建镜像） |

安装 GoReleaser：

```bash
# 任意平台（走 Go 工具链）
go install github.com/goreleaser/goreleaser/v2@latest

# macOS
brew install goreleaser

# Windows（任选其一）
scoop install goreleaser
choco install goreleaser

# 验证
goreleaser --version
```

安装 upx：

```bash
# Debian / Ubuntu
sudo apt-get install -y upx-ucl

# macOS
brew install upx

# Windows：从 https://upx.github.io 下载，解压后把 upx.exe 放进 PATH
```

> GitHub Actions 里 `upx` 与 `buildx` 都由 workflow 现装（`apt-get install upx-ucl` + `docker/setup-buildx-action`），你不需要在仓库里额外配置。

---

## 三、仓库里的相关文件

| 文件 | 作用 |
|---|---|
| `.goreleaser.yaml` | 全部打包/发布配置（本文重点） |
| `scripts/prepare-frontend.mjs` | 构建 Mellow → 同步到 `theme/dist`，供 `-tags embed` 使用 |
| `scripts/extract-release-notes.mjs` | 从 `CHANGELOG.md` 截取本版本段落，作为 Release 正文 |
| `Dockerfile` | 运行镜像（alpine + 非 root + 三个数据卷） |
| `.github/workflows/release.yml` | 推 `v*` tag 后自动跑完整流程 |

---

## 四、配置逐段说明

### 1. 构建前：先把前端做出来

```yaml
before:
  hooks:
    - node scripts/prepare-frontend.mjs
    - go mod download
```

`theme/theme_embed.go` 用 `//go:embed all:dist` 把 `theme/dist` 打进二进制，而这个目录不进版本库（已被 `.gitignore` 排除，构建时才生成）。所以编译前必须：

1. `npm run build`（产物在 `Mellow/dist`）
2. 清空 `theme/dist` 并把产物拷进去

这两步都在 `scripts/prepare-frontend.mjs` 里完成：没有 `node_modules` 时先 `npm ci`，构建后同步目录，产物缺失直接以非 0 退出（避免编出一个没有主题的包）。

> 只想同步已有产物、不重新构建：`node scripts/prepare-frontend.mjs --skip-build`

### 2. builds：内嵌前端 + 注入版本号

```yaml
builds:
  - id: inis
    main: .
    binary: inis
    tags:
      - embed              # 启用 theme_embed.go
    flags:
      - -trimpath
    env:
      - CGO_ENABLED=0      # 静态编译，镜像里不需要 libc
    ldflags:
      - -s -w -buildid=    # 去掉符号表 / 调试信息 / buildid
      - -X inis/app/facade.Version={{ .Version }}
    goos: [linux, windows, darwin]
    goarch: [amd64, arm64]
    mod_timestamp: '{{ .CommitTimestamp }}'
```

要点：

- **`tags: [embed]`**：不带这个标签编译出来的二进制没有主题，需要自己把 `Mellow/dist` 部署到 `public/`。
- **`-X inis/app/facade.Version=...`**：把 tag 版本写进二进制。`app/facade/const.go` 里的 `Version` 必须是 `var`（`const` 无法被 `-X` 覆盖），源码里的默认值只在你直接 `go build` 时生效。
- **`{{ .Version }}`** 是去掉 `v` 前缀的 tag，例如 `v1.0.9` → `1.0.9`。
- **`mod_timestamp`**：归档内文件时间统一取提交时间，同样的代码重复构建结果一致。

### 3. upx：把二进制压小

```yaml
upx:
  - enabled: true
    ids: [inis]
    goos: [linux, windows]   # macOS 不压：Mach-O 压缩后容易破坏签名/公证
    compress: best           # 等价 -9
    lzma: true
```

内嵌完整前端时单个二进制约 42 MB，UPX 压缩后再归档，最终下载体积约 12 MB。代价是启动时多几十毫秒（运行时自解压到内存），对常驻服务可忽略。

> Windows 的 exe 被 UPX 压缩后，杀毒软件误报率会略升。如果收到反馈，把 `goos` 改成 `[linux]` 即可。

### 4. archives / checksum

```yaml
archives:
  - id: release
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    files: [LICENSE, README.md, CHANGELOG.md]

checksum:
  name_template: checksums.txt
  algorithm: sha256
```

归档只放「二进制 + 3 个说明文件」，不会把整个仓库打进去。

### 5. dockers_v2：多架构镜像

```yaml
dockers_v2:
  - ids: [inis]
    dockerfile: Dockerfile
    disable: '{{ .IsSnapshot }}'     # 本地 snapshot 不构建镜像
    images:
      - ghcr.io/zhu885744/inis
    tags:
      - '{{ .Version }}'
      - '{{ if not .Prerelease }}latest{{ end }}'
    platforms:
      - linux/amd64
      - linux/arm64
```

**最容易踩的坑：构建上下文的目录结构。**

`dockers_v2` 是一次构建多平台，goreleaser 会把各平台二进制按 `os/arch` 分目录放好再交给 buildx：

```
/tmp/goreleaserdocker…/
├── Dockerfile
└── linux
    ├── amd64/inis
    └── arm64/inis
```

所以 `Dockerfile` 里不能写 `COPY inis`（根目录没有这个文件，会报 `"/inis": not found`），必须带上平台参数：

```dockerfile
FROM alpine:3.21
ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY ${TARGETOS}/${TARGETARCH}/inis /usr/local/bin/inis
```

`TARGETOS` / `TARGETARCH` 由 BuildKit 在跨平台构建时自动注入，等价的写法是 `COPY $TARGETPLATFORM/inis`。

镜像本身：alpine + `ca-certificates` + `tzdata`，非 root 用户 `inis`，`WORKDIR /app`，暴露 8642，声明 `/app/config`、`/app/runtime`、`/app/storage` 三个卷。

> 多架构构建需要 QEMU（Dockerfile 里的 `apk add` 要在 arm64 下执行），CI 已用 `docker/setup-qemu-action` 提供。

### 6. release / changelog：正文来自 CHANGELOG.md

```yaml
release:
  draft: false
  prerelease: auto
  name_template: 'INIS v{{ .Version }}'
  header: |
    > 单文件部署说明…
  footer: |
    完整更新日志见压缩包内的 `CHANGELOG.md`。
```

CI 在调用 goreleaser 前会先执行：

```bash
node scripts/extract-release-notes.mjs
# 从 CHANGELOG.md 截取 `## INIS-cms v1.0.9` 到下一个 `##` 之间的内容
# 输出到仓库根的 release-notes.md（已 gitignore）
```

然后：

```bash
goreleaser release --clean --release-notes=release-notes.md
```

`--release-notes` 存在时 goreleaser 不再用 git log 生成摘要；最终 Release 正文 = `header` + CHANGELOG 段落 + `footer`。

**因此 `header` 里不要再写 `## INIS v{{ .Version }}` 这样的标题**，否则会和 CHANGELOG 段落里的标题重复。

`changelog` 段保留着，只作为本地 `--snapshot` 试跑时的兜底。

---

## 五、本地验证（不发布）

```bash
# 1. 校验配置（不会编译）
goreleaser check

# 2. 完整跑一遍，产物落在 dist/，不推送任何东西
goreleaser release --snapshot --clean
```

snapshot 模式的差异：

- 版本号是 `{{ incpatch .Version }}-next`（例如最近 tag 是 `v1.0.9` → `1.0.10-next`），不会污染真实版本
- **不构建 Docker 镜像**（`disable: '{{ .IsSnapshot }}'`）
- 不创建 Release、不推送镜像

产物结构大致如下：

```
dist/
├── inis_1.0.10-next_linux_amd64.tar.gz
├── inis_1.0.10-next_windows_amd64.zip
├── …
└── checksums.txt
```

> 本地跑之前请确认 `upx` 在 PATH 里（除非临时把 `upx.enabled` 改成 false），否则会在压缩步骤失败。

---

## 六、正式发版流程

### 1. 先写 CHANGELOG

在 `CHANGELOG.md` 顶部加一段（标题格式必须能匹配版本号）：

```markdown
## INIS-cms v1.0.9
### 新增
- …

### 优化
- …

### 修复
- …
```

**这一步不能省**：`extract-release-notes.mjs` 找不到对应段落会以退出码 1 结束，整个流水线直接失败（故意设计成阻断，避免发出一个没有更新内容的 Release）。

本地想先跳过可用 `node scripts/extract-release-notes.mjs v1.0.9 --allow-missing`。

### 2. 提交代码

```bash
git add -A
git commit -m "release: v1.0.9"
```

> `git tag` 指向的是 HEAD。有未提交改动时打 tag，tag 里不会有这些改动。

### 3. 打 tag 并推送

```bash
git tag v1.0.9
git push origin main
git push origin v1.0.9      # 只推这个 tag；用 --tags 会把本地所有 tag 一起推
```

### 4. CI 自动执行

`.github/workflows/release.yml` 的步骤顺序：

```
checkout(fetch-depth 0)
  → setup-go(1.27.4) → setup-node(22)
  → QEMU / buildx / 登录 GHCR
  → govulncheck ./...            # 有漏洞直接中断（退出码 3）
  → 安装 upx
  → 从 CHANGELOG.md 提取发布说明
  → goreleaser release --clean --release-notes=release-notes.md
```

耗时参考：前端构建约 10 秒，6 平台编译 + UPX 约 1-2 分钟，Docker 双架构镜像 2-3 分钟，整体 5-8 分钟。

### 5. 验证

- GitHub Releases 页面：正文是否为 CHANGELOG 里的内容，资产是否为 6 个包 + `checksums.txt`
- `docker pull ghcr.io/zhu885744/inis:1.0.9` 能否拉取，`docker manifest inspect` 是否包含 amd64 / arm64

---

## 七、常见问题

| 现象 | 原因 | 处理 |
|---|---|---|
| `exit code 3`，日志里全是 `xxx calls yyy, which eventually calls zzz` | `govulncheck` 命中漏洞（退出码 3 即「发现漏洞」） | 按日志升级依赖（如 `go get golang.org/x/net@v0.60.0`），重新打 tag |
| `"/inis": not found` / `could not build docker image` | Dockerfile 的 `COPY` 没带平台目录 | 用 `COPY ${TARGETOS}/${TARGETARCH}/inis`，见第 4.5 节 |
| `CHANGELOG.md 里没有找到 vX.Y.Z 的段落` | 发版前没写 CHANGELOG | 补段落；段标题要与 tag 版本号一致 |
| 二进制里版本号还是旧值 | `Version` 被改回 `const`，或 tag 不在本次提交上 | 确认 `app/facade/const.go` 是 `var`；确认 tag 打在 HEAD |
| `no matching files found: all:dist` | `theme/dist` 为空，前端没构建 | 先手动跑 `node scripts/prepare-frontend.mjs` |
| `upx: command not found` | 构建机没装 upx | 安装 upx，或临时把 `upx.enabled` 改为 false |
| macOS 包比 Linux 大很多 | macOS 不启用 UPX（签名考虑） | 正常现象 |
| `git tag was not made against HEAD` | tag 打在旧提交上 | 删除重打：`git tag -f v1.0.9 <commit>` 后强推 tag |
| GHCR 拉取 403 / not found | 仓库私有，包默认也私有 | 到 GitHub Packages 里调整可见性，或先 `docker login ghcr.io` |

---

## 八、与 build.bat / build.sh 的区别

| | `build.bat` / `build.sh` | GoReleaser |
|---|---|---|
| 目标 | 本机快速出一个二进制 | 正式发版 |
| 平台 | 交互选择单平台或 6 平台批量 | 固定 6 个平台 |
| 前端内嵌 | 可选（询问 y/n） | 始终内嵌 |
| UPX | 可选（询问等级，macOS 跳过） | 固定 `-9 --lzma`，Linux/Windows |
| 版本号 | 取 `const.go` 里的默认值 | 由 tag 注入 |
| Docker 镜像 | 不支持 | 多架构推 GHCR |
| Release 说明 | 不支持 | 取自 `CHANGELOG.md` |
| 校验和 | 不支持 | `checksums.txt` |

日常开发用脚本，对外发版用 GoReleaser。

---

## 九、常见自定义

**增加目标平台**（例如 32 位 ARM）：

```yaml
builds:
  - goos: [linux, windows, darwin]
    goarch: [amd64, arm64, arm]
    goarm: [7]
```

注意：`dockers_v2.platforms` 目前只写 `linux/amd64`、`linux/arm64`，多出来的平台不会进镜像。

**换镜像仓库**：改 `dockers_v2.images` 与 workflow 里 `docker/login-action` 的 `registry`，同时给 workflow 配置对应的 `secrets`。

**关闭某个步骤**：`goreleaser release --skip=docker,archive`（可跳过的步骤见 `goreleaser release --help`）。

**只发 Docker 镜像不发压缩包**：`goreleaser release --skip=archive`。
