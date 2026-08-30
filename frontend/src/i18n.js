// 轻量国际化模块：按系统语言自动切换中英文，顶栏可手动切换
import { ref, computed } from 'vue'

const messages = {
  zh: {
    // 顶栏
    brandSub: '图标锻造 · PNG / JPG → ICO',
    changeImage: '更换图片',
    language: 'English',
    // 空状态
    loading: '正在读取…',
    selectImage: '点击选择图片',
    emptySub: '支持 PNG / JPG / BMP / GIF，建议使用 256×256 以上正方形图片',
    hintCorner: '自动识别圆角背景',
    hintCrop: '手动裁剪',
    hintMulti: '多尺寸 ICO 导出',
    // 圆角
    cornerCut: '圆角切割',
    cornerTip: '把圆角外的多余背景切除为透明，滑块向右切得更多',
    cornerDetected: '已自动识别圆角背景，建议半径 {r}px',
    restore: '恢复',
    cornerNotDetected: '未检测到圆角背景，请手动调整半径',
    radius: '半径',
    // 裁剪
    squareCrop: '正方形裁剪',
    cropBadge: '拖动画布调整位置',
    side: '边长',
    center: '居中',
    // ICO 尺寸
    icoSizes: 'ICO 尺寸',
    selectAll: '全选',
    deselectAll: '全不选',
    exportTip: '导出为 ZIP：含各尺寸独立 ICO（icon_16.ico…）+ 多尺寸合一的 icon.ico',
    // 导出
    exportBtn: '导出图标 ZIP',
    exporting: '正在生成…',
    // 预览
    sizePreview: '尺寸预览',
    // 提示
    needSelectSize: '请至少选择一个 ICO 尺寸',
    saved: '已保存：{path}',
  },
  en: {
    brandSub: 'Icon Foundry · PNG / JPG → ICO',
    changeImage: 'Change image',
    language: '中文',
    loading: 'Loading…',
    selectImage: 'Click to select an image',
    emptySub: 'Supports PNG / JPG / BMP / GIF. A 256×256 or larger square image is recommended.',
    hintCorner: 'Auto-detect corner background',
    hintCrop: 'Manual crop',
    hintMulti: 'Multi-size ICO export',
    cornerCut: 'Rounded Corners',
    cornerTip: 'Cut the extra background outside rounded corners to transparent. Slide right to cut away more.',
    cornerDetected: 'Corner background detected. Suggested radius {r}px.',
    restore: 'Reset',
    cornerNotDetected: 'No corner background detected. Adjust the radius manually.',
    radius: 'Radius',
    squareCrop: 'Square Crop',
    cropBadge: 'Drag on canvas to reposition',
    side: 'Size',
    center: 'Center',
    icoSizes: 'ICO Sizes',
    selectAll: 'Select all',
    deselectAll: 'Deselect all',
    exportTip: 'Exports a ZIP: one ICO per size (icon_16.ico…) plus a combined multi-size icon.ico',
    exportBtn: 'Export Icon ZIP',
    exporting: 'Generating…',
    sizePreview: 'Preview',
    needSelectSize: 'Select at least one ICO size first',
    saved: 'Saved: {path}',
  },
}

// 初始语言：优先 localStorage，否则探测系统语言
function detectLang() {
  const saved = localStorage.getItem('iconforge_lang')
  if (saved === 'zh' || saved === 'en') return saved
  return (navigator.language || 'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

const lang = ref(detectLang())

function applyLang(l) {
  document.documentElement.lang = l
  document.title = l === 'zh' ? 'IconForge · 图标锻造' : 'IconForge · Icon Foundry'
  try {
    localStorage.setItem('iconforge_lang', l)
  } catch (e) { /* 忽略存储异常 */ }
}
applyLang(lang.value)

function setLang(l) {
  lang.value = l
  applyLang(l)
}

// 模板中用到的 t(key[, params])：读取响应式 lang，切换时自动更新界面
function t(key, params) {
  let str = (messages[lang.value] && messages[lang.value][key]) || key
  if (params) {
    for (const k in params) {
      str = str.replace('{' + k + '}', String(params[k]))
    }
  }
  return str
}

const isZh = computed(() => lang.value === 'zh')

// 后端错误消息本地化：匹配已知中文前缀，英文环境下替换为英文
const errPrefixEn = {
  '读取文件失败': 'Failed to read file',
  '不支持的图片格式': 'Unsupported image format (PNG / JPG / BMP / GIF supported)',
  '不支持的文件类型': 'Unsupported file type',
  '图片数据无效': 'Invalid image data',
  '图片数据解码失败': 'Failed to decode image data',
  '图片解码失败': 'Failed to decode image',
  '生成 ICO 失败': 'Failed to generate ICO',
  '写入 ZIP 失败': 'Failed to write to ZIP',
  '打包 ZIP 失败': 'Failed to package ZIP',
  '写入文件失败': 'Failed to write file',
}

function tErr(msg) {
  if (lang.value !== 'en' || typeof msg !== 'string') return msg
  for (const k in errPrefixEn) {
    if (msg.startsWith(k)) {
      return msg.replace(k, errPrefixEn[k])
    }
  }
  // 单独处理 "生成 %dpx ICO 失败"
  const m = msg.match(/^生成 (\d+)px ICO 失败/)
  if (m) {
    return msg.replace('生成 ' + m[1] + 'px ICO 失败', 'Failed to generate ' + m[1] + 'px ICO')
  }
  return msg
}

export { lang, setLang, t, tErr, isZh }