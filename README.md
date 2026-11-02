# SysKline

把 Linux 系统资源变化放进交易终端。纯 Go + Bubble Tea + Lip Gloss，在 SSH
或本地终端查看实时 OHLC K 线，所有采集、交互和历史存储都在本机完成。

```text
 SYSKLINE  | linux-node                                    DEMO / LIVE  15:42:17
 CPU 47.6%  MEM 67.1%/32.0GiB  SWAP 6.6%  LOAD 2.48/2.53/2.01  UP 12d07h
[CPU] MEM  SWAP  DISK R  DISK W  NET RX  NET TX  LOAD 1  LOAD 5  LOAD 15
 1s  [5s]  15s  1m  5m  15m  1h  1d                              OHLC 12 bars  %
 CPU  47.6%  -5.98pp (-11.16%)  MA5 49.0%                              2.2s left
────────────────────────────────────────────────────────────────────────────────
   60.0 │┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄│┄┄┄┄┄┄┄┄┄┄┄┄
        │               ┊              ┊ │   ▄▄▄   │  ┊ │   ┌─┐   │
   50.0 │┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄│┄┄┄┌─┐┄┄▄▄▄┄┄▀▀▀┄┄███┄┄███┄┄└─┘┄┄███┄ C 47.6%
        │               ┊ │   ┌─┐  │ │ ··│····│··  │  ┊
   40.0 │┄┄┄┄┄┄┄│┄┄┄┄│┄┄┄┄│┄┄┄└─┘┄·└─┘·┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
   30.0 │┄┌─┐┄┄███┄·███··▀▀▀······┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
        │ └─┘···│··  │  ┊              ┊              ┊
   20.0 │┄┄│┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
        │               ┊              ┊              ┊
   10.0 │┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
         ────────────────────────────────────────────────────────────
         15:41:20  15:41:35       15:41:50       15:42:05    15:42:20
────────────────────────────────────────────────────────────────────────────────
 O 48.4%   H 55.4%   L 47.2%   C 47.6%
 11/02 15:42:15 FORMING  n=12  -0.82pp (-1.70%)  R 8.1%
 DISK R 5.3MiB/s W 2.0MiB/s  NET RX 10.3MiB/s TX 1.3MiB/s  tick 1s
 SYNTHETIC DEMO DATA  |  Hollow = rise  Solid = fall  |  ? help  q quit
 Tab metric  1-8 / [ ] period  <- -> cursor  +/- zoom  l line  Space pause
```

以上为实际 80×24 演示视图的无色文本帧。默认 **红涨绿跌**：上涨空心、下跌实心，
细线为真实高低影线；平盘使用十字线。建议 120×32 以上获得更清楚的蜡烛细节。

## 快速开始

需要 Go **1.25+**。运行平台为 Linux x86_64 / ARM64，不需要 root、CGO、系统
SQLite 库、浏览器或图形桌面。建议 80×24 以上终端；较小窗口会裁剪或显示缩放提示。

```sh
git clone https://github.com/AhsokaTano26/SysKline.git
cd SysKline
make build
./bin/syskline
```

安装到 `~/.local/bin`：

```sh
make install
export PATH="$HOME/.local/bin:$PATH"
syskline
```

也可以 `make install PREFIX=/usr/local`，目标目录需要当前用户有写入权限。
卸载只需删除安装的 `syskline` 文件，历史和配置保留。

在 macOS 开发环境或暂时没有 Linux 时，演示模式可验证完整 TUI：

```sh
./bin/syskline --demo
./bin/syskline --demo --ascii --no-color
```

演示模式明确标记 **DEMO / SYNTHETIC**，预置 8 天合成历史，始终使用内存数据库，
退出即丢弃；不允许与 `--db` 混用，不会污染真实指标历史。

## MVP 功能

- 十项指标：CPU 使用率、内存占比、Swap 占比、磁盘读/写字节速率、网络接收/发送
  字节速率、1/5/15 分钟系统负载。
- 1s、5s、15s、1m、5m、15m、1h、1d K 线，默认 **5 秒**；宽实体、细影线、
  半格高度精度、圆整数值刻度、秒级时间轴及最新收盘价标记。
- MA5 均线、选中收盘价十字光标、相对涨跌幅、区间振幅、样本数和当前 K 线倒计时。
- 历史滚动、光标选择、可见根数缩放、实时跟随、冻结视口、终端尺寸自适应。
- K 线与原始采样折线切换；折线取各时间箱最后一个真实样本，最多一个点/可用终端列。
- SQLite 原始指标与分钟 OHLC 持久化；自动聚合、独立保留期及定时清理。
- TOML 配置、配色、ASCII 模式、无色模式、JSON 快照。

## 快捷键

| 按键 | 操作 |
| --- | --- |
| `Tab` / `Shift+Tab`，`↓` / `↑` | 下一项 / 上一项指标 |
| `[` / `]` | 上一 / 下一 K 线周期 |
| `1` … `8` | 直接切换 1s / 5s / 15s / 1m / 5m / 15m / 1h / 1d |
| `←` / `→` | 逐根选择 K 线，到达边缘后滚动 |
| `PgUp` / `PgDn` | 向前 / 向后滚动半屏 |
| `+` / `-` | 缩小 / 扩大可见时间范围 |
| `Home` / `End` / `g` | 回到最新数据，恢复实时跟随 |
| `Space` | 冻结当前视口 / 恢复实时跟随 |
| `l` | K 线 / 折线 |
| `m` | 开关 MA5（连续 5 根 K 线收盘价的均值） |
| `a` | Unicode / ASCII |
| `r` | 重新加载当前视口历史 |
| `?` / `F1`，`Esc` | 帮助页 / 关闭帮助 |
| `q` / `Ctrl+C` | 退出并恢复终端 |

查看历史、暂停和打开帮助时仍会采集和保存新数据。光标选择的是时间桶，
缺失时段会显示没有 K 线。OHLC 详情显示所选桶的样本数及是否仍未收盘。

## 配置与命令行

配置文件默认位于 `$XDG_CONFIG_HOME/syskline/config.toml`，没有设置 XDG 时使用
`~/.config/syskline/config.toml`（Linux）。配置缺省时直接使用内置默认值。

```sh
mkdir -p ~/.config/syskline
cp configs/config.example.toml ~/.config/syskline/config.toml
syskline --config ~/.config/syskline/config.toml
syskline --metric net_rx --period 5m --interval 2s
syskline --period 1s          # 自动按 250ms 采样，约 4 个点形成一根秒 K 线
syskline --period 5s          # 默认周期，每秒采样，约 5 个点形成一根 K 线
syskline --db /path/to/history.db
syskline --ascii --no-color
syskline --once --interval 1s
syskline --help
syskline --version
```

显式指定但不存在的配置文件、未知字段或非法值会报错。命令行覆盖配置文件。
`NO_COLOR=1` 禁用颜色；`TERM=dumb` 默认启用 ASCII 和无色，可通过显式参数覆盖。
终端背景由终端自身控制，默认配色适合深色背景；配置中的颜色使用 `#RRGGBB`。
不自动改写配置文件，交互中的切换也不会保存到配置。

`--once` 输出含指标名、单位、时间戳、内存总量、Swap 总量、uptime 的 JSON，
无需交互式终端且不写数据库。CPU、I/O 速率需要两次读取，首次输出会等待一个采样
间隔。`--demo --once` 立即输出合成快照。普通交互模式要求 stdin/stdout 都是终端。

`interval` 是基础采样间隔。交互模式实际间隔为 `min(interval, 周期 / 4)`，最低
250ms；默认 5s 周期仍每秒采样，切到 1s 后自动切换为 250ms。切换周期会更新计时器，
无需等待旧的长间隔。底部 `tick` 显示实际间隔；`--once` 使用指定的基础间隔。

报价行涨跌相对上一根收盘价；OHLC 详情中的涨跌相对当前开盘价，CPU/内存等的
绝对变化使用 `pp`（百分点）。MA5 随当前未收盘 K 线更新，缺失时间桶会中断均线，
缩放到 4 根时也会读取视口前的历史用于计算。

## 指标语义与历史

- CPU = 两次 `/proc/stat` 读取间非 idle、非 iowait 时间占比；guest 不重复累计。
  这是主机汇总利用率，不按逻辑核数倍增。
- 内存 = `(MemTotal - MemAvailable) / MemTotal`；老内核使用 free/buffers/cache
  近似可用内存。Swap 未配置时显示 0%。
- 磁盘按 `/proc/diskstats` 扇区差分 × 512 字节计算。默认排除分区及 loop、ram、
  dm、md、zram，避免整盘/分区及映射设备重复统计；可用 `disks = ["dm-0"]` 等显式选择。
- 网络默认汇总 `/proc/net/dev` 中除 `lo` 外的接口。虚拟网卡、桥接和 veth 可能
  重复经过同一流量；需要准确的出口流量时配置 `interfaces = ["eth0"]`。
- 首次读取建立速率基线；计数器重置或设备新增不产生流量尖峰。负载是队列负载值，
  不是百分比。容器中部分 `/proc` 数据反映宿主机，并非 cgroup 配额利用率。
- 必需的 CPU/内存源失败时保留最近有效值、显示错误并重试；可选源读取失败时显示警告，
  对应指标显示 `n/a`、JSON 为 `null`、原始数据为 NULL，并跳过该指标的 K 线更新。
  速率尚无基线或计数器重置时也按缺失处理，避免给 K 线制造假的零值。

默认数据库：`$XDG_DATA_HOME/syskline/history.db`，否则为
`~/.local/share/syskline/history.db`。父目录和新文件分别以 0700 / 0600 创建。
SQLite 使用 WAL、NORMAL 同步和 4 MiB 页缓存，不需要单独运行数据库服务。

每次采样以一个事务写入原始点并更新 10 项指标的分钟 OHLC。长周期从分钟 OHLC
聚合；开收盘按真实时间排序，H/L 为实际极值。时间桶按 UTC 对齐，横轴按本地时间显示。
当前桶显示 **FORMING** 和剩余秒数；缺失样本不补零，缺失时间保留空白。
1s / 5s / 15s 从保留的原始毫秒样本聚合，不会由分钟 OHLC 重建。此前采样稀疏时，
对应秒桶可能只有一个样本，表现为真实平盘十字线；程序不会伪造实体或影线。

默认保留原始点 **7 天**、分钟 K 线 **90 天**，启动和每小时执行清理，使用
incremental vacuum 分批回收空页。每秒采样的原始数据约 60 万行/7 天，分钟数据约
130 万行/90 天；持续 250ms 采样约 242 万原始行/7 天。磁盘占用取决于 SQLite 页面
与实际采样时间。秒 K 线和折线只能读取原始保留期内的数据，分钟及更长 OHLC 仍可查。
`raw_days <= history_days`，均可配置。

**仅在进程运行期间记录数据**，退出后不会继续采集；重新启动会读取历史。不包含
守护进程、告警、远程采集、单进程列表或单核视图。MVP 以单台主机、单个采集实例
对应一个数据库为使用方式，跨机器复制的数据库不应继续混合写入。

## GitHub Actions 下载与发布

打开仓库的 [Actions 页面](https://github.com/AhsokaTano26/SysKline/actions/workflows/ci.yml)，
选择 **Build binaries**：每次推送 `main` 或 `v*` 标签都会运行；也可点击
**Run workflow** 手动构建，`version` 可留空（自动生成 `dev-提交号`）或填写版本号。

构建先运行测试、竞态检测和真实终端采集检查，通过后生成 Linux **amd64 / arm64**
静态可执行文件。进入成功的运行记录，在 **Artifacts** 下载 `syskline-linux-版本`，
其中包含两个单文件二进制、两个安装压缩包及 `SHA256SUMS`，构建产物保留 30 天。

在 Linux 解压对应架构的安装包即可运行，无需安装 Go：

```sh
# 将 VERSION 替换为实际下载文件名中的版本号；ARM64 改为 arm64。
tar -xzf syskline_VERSION_linux_amd64.tar.gz
./syskline --version
./syskline
```

每个安装包包含 `syskline`、MIT 许可证、README 和 `configs/config.example.toml`。
下载 Actions 产物外层 ZIP 后，可先运行 `sha256sum --check SHA256SUMS` 校验完整性。
直接使用原始 `syskline-linux-amd64` / `syskline-linux-arm64` 文件时需要 `chmod +x`；
安装压缩包会保留执行权限。

推送版本标签会在构建通过后自动创建 [GitHub Release](https://github.com/AhsokaTano26/SysKline/releases)
并上传全部二进制、安装包和校验文件。例如未来发布 `v0.1.0`：

```sh
git tag v0.1.0
git push origin v0.1.0
```

正式版本通过 Releases 长期下载，普通分支构建通过 Actions Artifacts 下载。

## 构建、交叉编译与验证

```sh
make check                  # 单元/集成测试 + go vet
make race                   # Go race detector（需要本机 C 编译器，仅测试使用）
make release VERSION=0.1.0  # Linux amd64 / arm64，CGO_ENABLED=0
python3 scripts/smoke_tui.py # Unix PTY 演示交互检查，仅用 Python 标准库
# Linux 上也可运行：python3 scripts/smoke_tui.py --live
```

发布产物为 `dist/syskline-linux-amd64`、`dist/syskline-linux-arm64`、
`syskline_版本_linux_amd64.tar.gz` / `syskline_版本_linux_arm64.tar.gz` 和 `SHA256SUMS`。
复制目标架构对应的一个可执行文件到 Linux 即可运行：

```sh
PREFIX="$HOME/.local" ./scripts/install.sh dist/syskline-linux-arm64
syskline
```

`./scripts/build.sh` 也可独立执行，构建无需交叉 C 编译器。`.github/workflows/ci.yml`
在 Linux 上运行测试、竞态检查、静态检查、双架构构建和真实采集快照。

代码按 `internal/metrics`（procfs 与演示采集）、`internal/store`（SQLite）、
`internal/chart`（单元格绘图）、`internal/tui`（状态与视图）、`internal/config`
及 `cmd/syskline` 分层。测试覆盖真实临时 SQLite、聚合、清理、采集文件夹具、
热插拔/计数器重置、视口导航、尺寸裁剪、ASCII 和 CLI 配置覆盖。

## 许可证

[MIT](LICENSE)
