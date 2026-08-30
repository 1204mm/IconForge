# IconForge

PNG / JPG 转 ICO 图标桌面工具。基于 Wails（Go + Vue3）构建，开箱即用，无需安装。

## 功能特性

- **多尺寸导出**：支持 16 / 24 / 32 / 48 / 64 / 128 / 256 px，自由勾选组合
- **圆角自动识别**：自动检测图片四角的多余背景（如带浅色圆角外框的图标图），一键切除
- **圆角手动微调**：统一滑块控制四角圆角半径，实时预览
- **手动裁剪**：拖拽正方形裁剪框，任意选取画面区域
- **圆角切割**：基于带符号距离场（SDF）的抗锯齿蒙版，边缘平滑无锯齿
- **高质量缩放**：Lanczos 算法，小尺寸图标依然清晰
- **ZIP 打包导出**：每个尺寸独立 ICO + 多尺寸合一 `icon.ico`，一次导出全部

## ICO 格式说明

为保证兼容性，导出采用业界标准做法（与 ImageMagick 等工具一致）：

| 尺寸 | 条目格式 | 说明 |
|------|----------|------|
| 16 ~ 128 px | 32bpp BMP | ICO 原始标准格式，所有查看器和旧版系统均支持 |
| 256 px | PNG 压缩 | 官方为大图标设计，体积小 |

多尺寸合一的 `icon.ico` 内部包含全部勾选尺寸，Windows 会按使用场景自动选用。

## 技术栈

- **后端**：Go + [Wails v2](https://wails.io)（桌面框架）+ [imaging](https://github.com/disintegration/imaging)（图像处理）
- **前端**：Vue3 + Vite，纯手写 CSS，无 UI 组件库
- **图像算法**：圆角识别（对角线扫描 + 边缘扫描联合估算 + 背景均匀性校验）、SDF 抗锯齿圆角蒙版、Lanczos 高质量缩放、ICO 编码（BMP + PNG 双格式）

## 构建与运行

环境要求：Go 1.21+、Node.js 16+、[Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# 安装 Wails CLI（已安装可跳过）
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 开发模式运行（热重载）
wails dev

# 打包生产版本（产物在 build/bin/）
wails build
```

运行测试：

```bash
go test -v ./...
```

## 项目结构

```
iconforge/
├── main.go               # Wails 应用入口（窗口配置）
├── app.go                # 后端绑定：文件选择、加载、ZIP 导出
├── imagetools.go         # 核心：圆角识别/切割、裁剪、缩放、ICO 编码
├── imagetools_test.go    # 单元测试
├── wails.json            # Wails 项目配置
└── frontend/             # Vue3 前端
    ├── index.html
    ├── vite.config.js
    └── src/
        ├── App.vue       # 编辑器界面（Canvas 裁剪 + 实时预览）
        ├── main.js
        └── style.css     # 全局样式（纯手写 CSS）
```

## 使用流程

1. 打开图片（支持 PNG / JPG / BMP / GIF，或直接拖入）
2. 工具自动识别圆角外背景，可拖动滑块微调切割半径
3. 需要时拖拽调整正方形裁剪框
4. 勾选所需尺寸
5. 点击"导出图标 ZIP"，得到各尺寸独立 ICO 与多合一 `icon.ico`

## License

MIT
