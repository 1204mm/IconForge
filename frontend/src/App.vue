<script setup>
import { ref, reactive, computed, watch, onMounted, nextTick } from 'vue'
import { OpenImageDialog, ExportIcons } from '../wailsjs/go/main/App'
import { t, tErr, setLang, isZh } from './i18n'

/* ---------- 状态 ---------- */
const imgInfo = ref(null)          // { name, width, height, dataUrl, detectedRadius, cornerDetected }
const imgEl = ref(null)            // HTMLImageElement
const loading = ref(false)
const exporting = ref(false)

const cornerOn = ref(false)        // 圆角切割开关
const radius = ref(0)              // 圆角半径（原图像素）
const crop = reactive({ x: 0, y: 0, size: 0 })

const SIZE_OPTIONS = [16, 24, 32, 48, 64, 128, 256]
const sizeSel = reactive({ 16: true, 24: true, 32: true, 48: true, 64: true, 128: true, 256: true })

const toast = ref(null)            // { type: 'ok'|'err', text }

/* ---------- 计算属性 ---------- */
const minSide = computed(() => imgInfo.value ? Math.min(imgInfo.value.width, imgInfo.value.height) : 0)
const maxRadius = computed(() => Math.floor(minSide.value / 2))
const selectedSizes = computed(() => SIZE_OPTIONS.filter(s => sizeSel[s]).sort((a, b) => a - b))
const allSelected = computed(() => selectedSizes.value.length === SIZE_OPTIONS.length)

/* ---------- 打开图片 ---------- */
async function openImage() {
  if (loading.value) return
  loading.value = true
  toast.value = null
  try {
    const info = await OpenImageDialog()
    if (info) applyImage(info)
  } catch (e) {
    showError(e)
  } finally {
    loading.value = false
  }
}

function applyImage(info) {
  imgInfo.value = info
  const im = new Image()
  im.onload = () => {
    imgEl.value = im
    // 默认裁剪：最大正方形居中
    crop.size = Math.min(info.width, info.height)
    crop.x = Math.floor((info.width - crop.size) / 2)
    crop.y = Math.floor((info.height - crop.size) / 2)
    // 自动圆角
    if (info.cornerDetected) {
      cornerOn.value = true
      radius.value = Math.min(info.detectedRadius, Math.floor(crop.size / 2))
    } else {
      cornerOn.value = false
      radius.value = 0
    }
    nextTick(redrawAll)
  }
  im.src = info.dataUrl
}

function showError(e) {
  const msg = typeof e === 'string' ? e : (e.message || String(e))
  toast.value = { type: 'err', text: tErr(msg) }
}

/* ---------- Canvas 编辑器 ---------- */
const canvasRef = ref(null)
let dragState = null

function draw() {
  const cv = canvasRef.value
  const im = imgEl.value
  if (!cv || !im || !imgInfo.value) return

  const wrap = cv.parentElement
  const dpr = window.devicePixelRatio || 1
  const cw = wrap.clientWidth
  const ch = wrap.clientHeight
  cv.width = cw * dpr
  cv.height = ch * dpr
  cv.style.width = cw + 'px'
  cv.style.height = ch + 'px'
  const ctx = cv.getContext('2d')
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cw, ch)

  // 图片适配缩放
  const scale = Math.min(cw / im.naturalWidth, ch / im.naturalHeight, 1)
  const dw = im.naturalWidth * scale
  const dh = im.naturalHeight * scale
  const ox = (cw - dw) / 2
  const oy = (ch - dh) / 2

  // 裁剪框（画布坐标）
  const cs = crop.size * scale
  const cx = ox + crop.x * scale
  const cy = oy + crop.y * scale
  const cr = cornerOn.value ? Math.min(radius.value * scale, cs / 2) : 0

  // 1. 整图压暗
  ctx.globalAlpha = 0.25
  ctx.drawImage(im, ox, oy, dw, dh)
  ctx.globalAlpha = 1

  // 2. 裁剪区：棋盘格背景
  const pattern = getCheckerPattern(ctx)
  ctx.save()
  roundRectPath(ctx, cx, cy, cs, cs, cr)
  ctx.clip()
  ctx.fillStyle = pattern
  ctx.fillRect(cx, cy, cs, cs)
  // 3. 裁剪区内重绘原图（从裁剪框源区域裁出，与导出一致）
  ctx.drawImage(im, crop.x, crop.y, crop.size, crop.size, cx, cy, cs, cs)
  ctx.restore()

  // 4. 裁剪框描边
  ctx.save()
  roundRectPath(ctx, cx, cy, cs, cs, cr)
  ctx.strokeStyle = 'rgba(245,165,36,0.95)'
  ctx.lineWidth = 1.5
  ctx.setLineDash([])
  ctx.stroke()
  ctx.setLineDash([4, 4])
  ctx.strokeStyle = 'rgba(0,0,0,0.5)'
  ctx.lineWidth = 1
  ctx.stroke()
  ctx.restore()

  // 5. 四角手柄
  const hs = 5
  ctx.fillStyle = '#f5a524'
  for (const [hx, hy] of cornerPoints(cx, cy, cs, cr)) {
    ctx.beginPath()
    ctx.arc(hx, hy, hs, 0, Math.PI * 2)
    ctx.fill()
    ctx.strokeStyle = '#141519'
    ctx.lineWidth = 1.5
    ctx.stroke()
  }
}

function cornerPoints(x, y, s, r) {
  // 圆弧上的 45° 点作为手柄位置
  const k = r * (1 - Math.SQRT1_2)
  return [
    [x + k, y + k], [x + s - k, y + k],
    [x + k, y + s - k], [x + s - k, y + s - k],
  ]
}

function roundRectPath(ctx, x, y, w, h, r) {
  r = Math.min(r, w / 2, h / 2)
  ctx.beginPath()
  if (r <= 0) {
    ctx.rect(x, y, w, h)
    return
  }
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

let checkerPattern = null
function getCheckerPattern(ctx) {
  if (checkerPattern) return checkerPattern
  const t = document.createElement('canvas')
  t.width = 16; t.height = 16
  const tc = t.getContext('2d')
  tc.fillStyle = '#262a33'
  tc.fillRect(0, 0, 16, 16)
  tc.fillStyle = '#1b1e25'
  tc.fillRect(0, 0, 8, 8)
  tc.fillRect(8, 8, 8, 8)
  checkerPattern = ctx.createPattern(t, 'repeat')
  return checkerPattern
}

/* ---------- 拖动裁剪框 ---------- */
function onPointerDown(e) {
  if (!imgEl.value || !canvasRef.value) return
  const cv = canvasRef.value
  const rect = cv.getBoundingClientRect()
  const px = e.clientX - rect.left
  const py = e.clientY - rect.top

  const { cx, cy, cs } = cropCanvasRect(cv)
  // 点在裁剪框内 -> 开始拖动，记录按下时的初始位置作为稳定基准
  if (px >= cx - 4 && px <= cx + cs + 4 && py >= cy - 4 && py <= cy + cs + 4) {
    dragState = { startX: crop.x, startY: crop.y, pointerX: px, pointerY: py }
    cv.setPointerCapture(e.pointerId)
    e.preventDefault()
  }
}

function onPointerMove(e) {
  const cv = canvasRef.value
  if (!cv || !imgEl.value) return
  const rect = cv.getBoundingClientRect()
  const px = e.clientX - rect.left
  const py = e.clientY - rect.top

  if (!dragState) {
    // 更新鼠标样式
    const { cx, cy, cs } = cropCanvasRect(cv)
    const inside = px >= cx && px <= cx + cs && py >= cy && py <= cy + cs
    cv.style.cursor = inside ? 'move' : 'default'
    return
  }

  const { cs } = cropCanvasRect(cv)
  const imgScale = crop.size / cs
  // 用按下时的初始位置 + 鼠标位移增量，避免反馈振荡
  const nx = dragState.startX + (px - dragState.pointerX) * imgScale
  const ny = dragState.startY + (py - dragState.pointerY) * imgScale
  clampCrop(nx, ny)
  draw()
}

function onPointerUp() {
  dragState = null
}

function cropCanvasRect(cv) {
  const im = imgEl.value
  const cw = cv.clientWidth
  const ch = cv.clientHeight
  const scale = Math.min(cw / im.naturalWidth, ch / im.naturalHeight, 1)
  const dw = im.naturalWidth * scale
  const dh = im.naturalHeight * scale
  const ox = (cw - dw) / 2
  const oy = (ch - dh) / 2
  return { cx: ox + crop.x * scale, cy: oy + crop.y * scale, cs: crop.size * scale }
}

function clampCrop(nx, ny) {
  const w = imgInfo.value.width
  const h = imgInfo.value.height
  crop.x = Math.round(Math.max(0, Math.min(nx, w - crop.size)))
  crop.y = Math.round(Math.max(0, Math.min(ny, h - crop.size)))
}

function centerCrop() {
  const w = imgInfo.value.width
  const h = imgInfo.value.height
  crop.x = Math.floor((w - crop.size) / 2)
  crop.y = Math.floor((h - crop.size) / 2)
}

/* ---------- 尺寸预览 ---------- */
const previewRefs = ref([])
function drawPreview(s, idx) {
  const cv = previewRefs.value[idx]
  const im = imgEl.value
  if (!cv || !im) return
  const dpr = window.devicePixelRatio || 1
  cv.width = s * dpr
  cv.height = s * dpr
  cv.style.width = s + 'px'
  cv.style.height = s + 'px'
  const ctx = cv.getContext('2d')
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, s, s)
  const r = cornerOn.value ? Math.min(radius.value * s / crop.size, s / 2) : 0
  ctx.save()
  roundRectPath(ctx, 0, 0, s, s, r)
  ctx.clip()
  ctx.imageSmoothingQuality = 'high'
  ctx.drawImage(im, crop.x, crop.y, crop.size, crop.size, 0, 0, s, s)
  ctx.restore()
}

function redrawAll() {
  draw()
  PREVIEW_SIZES.forEach((s, i) => drawPreview(s, i))
}

const PREVIEW_SIZES = [16, 24, 32, 48, 64, 128]

watch([radius, cornerOn, () => crop.x, () => crop.y, () => crop.size], redrawAll)

/* ---------- 尺寸选择 ---------- */
function toggleAll() {
  const v = !allSelected.value
  SIZE_OPTIONS.forEach(s => { sizeSel[s] = v })
}

/* ---------- 导出 ---------- */
async function doExport() {
  if (!imgInfo.value || exporting.value) return
  if (selectedSizes.value.length === 0) {
    toast.value = { type: 'err', text: t('needSelectSize') }
    return
  }
  exporting.value = true
  toast.value = null
  try {
    const path = await ExportIcons({
      imageData: imgInfo.value.dataUrl,
      cropX: crop.x,
      cropY: crop.y,
      cropSize: crop.size,
      cornerRadius: cornerOn.value ? radius.value : 0,
      sizes: selectedSizes.value,
    })
    if (path) {
      toast.value = { type: 'ok', text: t('saved', { path }) }
    }
  } catch (e) {
    showError(e)
  } finally {
    exporting.value = false
  }
}

/* ---------- 其它 ---------- */
onMounted(() => {
  window.addEventListener('resize', redrawAll)
})
</script>

<template>
  <div class="app">
    <!-- 顶栏 -->
    <header class="topbar">
      <div class="brand">
        <svg class="brand-mark" viewBox="0 0 24 24" fill="none">
          <defs>
            <linearGradient id="lg" x1="0" y1="0" x2="24" y2="24">
              <stop offset="0" stop-color="#f5a524" />
              <stop offset="1" stop-color="#fb923c" />
            </linearGradient>
          </defs>
          <path d="M4 20h9v-2.5H4V20zm14.8-12.6-2.6-2.6a1.2 1.2 0 0 0-1.7 0l-1.2 1.2 4.3 4.3 1.2-1.2a1.2 1.2 0 0 0 0-1.7zM10.2 8.9 4 15.1v3h3l6.2-6.2-3-3z" fill="url(#lg)"/>
        </svg>
        <div class="brand-text">
          <span class="brand-name">IconForge</span>
          <span class="brand-sub">{{ t('brandSub') }}</span>
        </div>
      </div>
      <div v-if="imgInfo" class="file-meta">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="3" width="18" height="18" rx="2"/>
          <circle cx="8.5" cy="8.5" r="1.5"/>
          <path d="m21 15-5-5L5 21"/>
        </svg>
        <span class="file-name" :title="imgInfo.name">{{ imgInfo.name }}</span>
        <span class="file-dim">{{ imgInfo.width }} × {{ imgInfo.height }}</span>
        <button class="btn-ghost" @click="openImage">{{ t('changeImage') }}</button>
      </div>
      <button class="btn-lang" :title="t('language')" @click="setLang(isZh ? 'en' : 'zh')">{{ isZh ? 'EN' : '中' }}</button>
    </header>

    <!-- 空状态 -->
    <div v-if="!imgInfo" class="empty">
      <div class="dropzone" @click="openImage">
        <svg viewBox="0 0 24 24" width="56" height="56" fill="none" stroke="var(--accent)" stroke-width="1.3">
          <rect x="3" y="5" width="18" height="16" rx="3"/>
          <circle cx="9" cy="10" r="1.6"/>
          <path d="m21 16-4.5-4.5L7 21"/>
        </svg>
        <div class="dz-title">{{ loading ? t('loading') : t('selectImage') }}</div>
        <div class="dz-sub">{{ t('emptySub') }}</div>
        <div class="dz-hint">
          <span>{{ t('hintCorner') }}</span><i></i><span>{{ t('hintCrop') }}</span><i></i><span>{{ t('hintMulti') }}</span>
        </div>
      </div>
    </div>

    <!-- 编辑器 -->
    <div v-else class="editor">
      <!-- 左：画布 -->
      <div class="canvas-wrap">
        <div class="canvas-holder">
          <canvas ref="canvasRef"
                  @pointerdown="onPointerDown"
                  @pointermove="onPointerMove"
                  @pointerup="onPointerUp"></canvas>
        </div>
        <div class="preview-bar">
          <span class="pb-label">{{ t('sizePreview') }}</span>
          <div class="pb-items">
            <div v-for="(s, i) in PREVIEW_SIZES" :key="s" class="pb-item">
              <div class="pb-canvas checkerboard"><canvas :ref="el => previewRefs[i] = el"></canvas></div>
              <span class="pb-size">{{ s }}px</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 右：参数面板 -->
      <aside class="panel">
        <!-- 圆角切割 -->
        <section class="sec">
          <div class="sec-head">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M4 8a4 4 0 0 1 4-4h12v12a4 4 0 0 1-4 4H4V8z"/>
            </svg>
            <span>{{ t('cornerCut') }}</span>
            <label class="switch">
              <input type="checkbox" v-model="cornerOn">
              <span class="slider"></span>
            </label>
          </div>
          <template v-if="cornerOn">
            <div v-if="imgInfo.cornerDetected" class="auto-badge">
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M13 2 3 14h7l-1 8 10-12h-7l1-8z"/>
              </svg>
              {{ t('cornerDetected', { r: imgInfo.detectedRadius }) }}
              <button v-if="radius !== imgInfo.detectedRadius" class="link" @click="radius = Math.min(imgInfo.detectedRadius, maxRadius)">{{ t('restore') }}</button>
            </div>
            <div v-else class="auto-badge dim">
              {{ t('cornerNotDetected') }}
            </div>
            <div class="slider-row">
              <span class="slider-label">{{ t('radius') }}</span>
              <input type="range" min="0" :max="maxRadius" step="1" v-model.number="radius">
              <span class="slider-val">{{ radius }}px</span>
            </div>
          </template>
          <p class="sec-tip">{{ t('cornerTip') }}</p>
        </section>

        <!-- 手动裁剪 -->
        <section class="sec">
          <div class="sec-head">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M6 2v14a2 2 0 0 0 2 2h14"/>
              <path d="M18 22V8a2 2 0 0 0-2-2H2"/>
            </svg>
            <span>{{ t('squareCrop') }}</span>
            <span class="sec-badge">{{ t('cropBadge') }}</span>
          </div>
          <div class="slider-row">
            <span class="slider-label">{{ t('side') }}</span>
            <input type="range" min="64" :max="minSide" step="1" v-model.number="crop.size"
                   @input="clampCrop(crop.x, crop.y)">
            <span class="slider-val">{{ crop.size }}px</span>
          </div>
          <div class="crop-xy">
            <span>X <b>{{ crop.x }}</b></span>
            <span>Y <b>{{ crop.y }}</b></span>
            <button class="btn-mini" @click="centerCrop">{{ t('center') }}</button>
          </div>
        </section>

        <!-- ICO 尺寸 -->
        <section class="sec">
          <div class="sec-head">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="7" height="7" rx="1"/>
              <rect x="14" y="3" width="7" height="7" rx="1"/>
              <rect x="3" y="14" width="7" height="7" rx="1"/>
              <rect x="14" y="14" width="7" height="7" rx="1"/>
            </svg>
            <span>{{ t('icoSizes') }}</span>
            <button class="link" @click="toggleAll">{{ allSelected ? t('deselectAll') : t('selectAll') }}</button>
          </div>
          <div class="size-grid">
            <label v-for="s in SIZE_OPTIONS" :key="s" class="size-chip" :class="{ on: sizeSel[s] }">
              <input type="checkbox" v-model="sizeSel[s]">
              <span>{{ s }}×{{ s }}</span>
            </label>
          </div>
          <p class="sec-tip">{{ t('exportTip') }}</p>
        </section>

        <!-- 导出 -->
        <div class="export-zone">
          <button class="btn-export" :disabled="exporting || selectedSizes.length === 0" @click="doExport">
            <svg v-if="!exporting" viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <path d="m7 10 5 5 5-5"/>
              <path d="M12 15V3"/>
            </svg>
            <span v-else class="spinner"></span>
            {{ exporting ? t('exporting') : t('exportBtn') }}
          </button>
        </div>
      </aside>
    </div>

    <!-- Toast -->
    <transition name="toast">
      <div v-if="toast" class="toast" :class="toast.type">
        <svg v-if="toast.type === 'ok'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.5">
          <path d="M20 6 9 17l-5-5"/>
        </svg>
        <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.5">
          <circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/>
        </svg>
        <span>{{ toast.text }}</span>
        <button class="toast-close" @click="toast = null">×</button>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.app { height: 100%; display: flex; flex-direction: column; }

/* ---------- 顶栏 ---------- */
.topbar {
  height: 52px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px;
  background: linear-gradient(180deg, #1a1c22, #171920);
  border-bottom: 1px solid var(--border-soft);
}
.brand { display: flex; align-items: center; gap: 10px; }
.brand-mark { width: 26px; height: 26px; filter: drop-shadow(0 0 8px rgba(245,165,36,.35)); }
.brand-text { display: flex; align-items: baseline; gap: 10px; }
.brand-name {
  font-size: 17px; font-weight: 700; letter-spacing: .3px;
  background: linear-gradient(90deg, #fbbf24, #fb923c);
  -webkit-background-clip: text; background-clip: text; color: transparent;
}
.brand-sub { font-size: 11.5px; color: var(--text-faint); }

.file-meta { display: flex; align-items: center; gap: 10px; color: var(--text-dim); font-size: 12.5px; }
.file-name { max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); }
.file-dim { font-family: var(--mono); font-size: 11.5px; color: var(--text-faint); }

/* ---------- 空状态 ---------- */
.empty { flex: 1; display: flex; align-items: center; justify-content: center; }
.dropzone {
  width: 460px; max-width: 86%;
  padding: 52px 40px 44px;
  text-align: center;
  border: 1.5px dashed var(--border);
  border-radius: 16px;
  background: radial-gradient(ellipse at 50% 0%, rgba(245,165,36,.05), transparent 65%), var(--bg-panel);
  cursor: pointer;
  transition: border-color .2s, background .2s, transform .2s;
}
.dropzone:hover {
  border-color: var(--accent-border);
  transform: translateY(-2px);
  box-shadow: 0 12px 40px rgba(0,0,0,.4);
}
.dz-title { font-size: 18px; font-weight: 600; margin-top: 18px; }
.dz-sub { font-size: 12.5px; color: var(--text-dim); margin-top: 8px; }
.dz-hint { display: flex; align-items: center; justify-content: center; gap: 10px; margin-top: 22px; font-size: 11.5px; color: var(--text-faint); }
.dz-hint i { width: 3px; height: 3px; border-radius: 50%; background: var(--text-faint); }

/* ---------- 编辑器布局 ---------- */
.editor { flex: 1; display: flex; min-height: 0; }

.canvas-wrap { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.canvas-holder {
  flex: 1; min-height: 0;
  margin: 14px 14px 8px 14px;
  border: 1px solid var(--border-soft);
  border-radius: 12px;
  background:
    radial-gradient(ellipse at 30% 0%, rgba(255,255,255,.025), transparent 60%),
    var(--bg-panel);
  display: flex; align-items: center; justify-content: center;
  overflow: hidden;
  position: relative;
}

.preview-bar {
  flex-shrink: 0;
  display: flex; align-items: center; gap: 14px;
  padding: 10px 16px 14px;
}
.pb-label { font-size: 11px; color: var(--text-faint); letter-spacing: 1px; writing-mode: vertical-lr; }
.pb-items { display: flex; align-items: flex-end; gap: 16px; }
.pb-item { display: flex; flex-direction: column; align-items: center; gap: 5px; }
.pb-canvas { border-radius: 4px; overflow: hidden; display: flex; align-items: center; justify-content: center; }
.pb-size { font-family: var(--mono); font-size: 10px; color: var(--text-faint); }

/* ---------- 参数面板 ---------- */
.panel {
  width: 302px; flex-shrink: 0;
  margin: 14px; margin-left: 0;
  border: 1px solid var(--border-soft);
  border-radius: 12px;
  background: var(--bg-panel);
  padding: 4px 0;
  overflow-y: auto;
  display: flex; flex-direction: column;
}
.sec { padding: 16px 18px 14px; border-bottom: 1px solid var(--border-soft); }
.sec-head { display: flex; align-items: center; gap: 8px; font-size: 13.5px; font-weight: 600; margin-bottom: 12px; }
.sec-head svg { color: var(--accent); flex-shrink: 0; }
.sec-head > span:first-of-type { flex: 1; }
.sec-badge { font-size: 10.5px; font-weight: 400; color: var(--text-faint); }
.sec-tip { font-size: 11px; color: var(--text-faint); margin-top: 10px; line-height: 1.5; }

.auto-badge {
  display: flex; align-items: center; gap: 6px; flex-wrap: wrap;
  font-size: 11.5px; color: var(--ok);
  background: rgba(74,222,128,.08);
  border: 1px solid rgba(74,222,128,.2);
  border-radius: 6px; padding: 7px 9px; margin-bottom: 12px;
}
.auto-badge.dim { color: var(--text-dim); background: var(--bg-panel-2); border-color: var(--border-soft); }
.auto-badge svg { flex-shrink: 0; color: var(--ok); }
.auto-badge.dim svg { color: var(--text-faint); }

/* 滑块行 */
.slider-row { display: flex; align-items: center; gap: 10px; }
.slider-label { font-size: 12px; color: var(--text-dim); width: 28px; flex-shrink: 0; }
.slider-val { font-family: var(--mono); font-size: 11.5px; color: var(--accent); width: 46px; text-align: right; flex-shrink: 0; }
input[type=range] {
  flex: 1; -webkit-appearance: none; appearance: none;
  height: 4px; border-radius: 2px;
  background: #33363f;
  outline: none; cursor: pointer;
}
input[type=range]::-webkit-slider-thumb {
  -webkit-appearance: none; appearance: none;
  width: 14px; height: 14px; border-radius: 50%;
  background: #fff; border: 3px solid var(--accent);
  box-shadow: 0 1px 4px rgba(0,0,0,.5);
  cursor: grab;
}

.crop-xy {
  display: flex; align-items: center; gap: 12px;
  margin-top: 11px; font-size: 11.5px; color: var(--text-dim);
  font-family: var(--mono);
}
.crop-xy b { color: var(--text); font-weight: 500; }

/* 开关 */
.switch { position: relative; width: 34px; height: 19px; flex-shrink: 0; cursor: pointer; }
.switch input { opacity: 0; width: 0; height: 0; }
.switch .slider {
  position: absolute; inset: 0;
  background: #33363f; border-radius: 10px;
  transition: background .18s;
}
.switch .slider::before {
  content: ''; position: absolute;
  width: 13px; height: 13px; border-radius: 50%;
  left: 3px; top: 3px; background: #9aa0ad;
  transition: transform .18s, background .18s;
}
.switch input:checked + .slider { background: var(--accent-soft); box-shadow: inset 0 0 0 1.5px var(--accent-border); }
.switch input:checked + .slider::before { transform: translateX(15px); background: var(--accent); }

/* 尺寸芯片 */
.size-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 7px; }
.size-chip {
  display: flex; align-items: center; justify-content: center;
  border: 1px solid var(--border); border-radius: 7px;
  padding: 7px 0; font-size: 12px; font-family: var(--mono);
  color: var(--text-dim); background: var(--bg-input);
  cursor: pointer; transition: all .15s;
}
.size-chip input { display: none; }
.size-chip:hover { border-color: var(--accent-border); color: var(--text); }
.size-chip.on {
  color: var(--accent); border-color: var(--accent-border);
  background: var(--accent-soft);
}

/* 按钮 */
.btn-ghost {
  border: 1px solid var(--border); background: var(--bg-panel-2);
  color: var(--text-dim); font-size: 11.5px;
  padding: 5px 11px; border-radius: 6px; cursor: pointer;
  transition: all .15s;
}
.btn-ghost:hover { color: var(--text); border-color: var(--accent-border); }

.btn-lang {
  flex-shrink: 0;
  min-width: 34px;
  border: 1px solid var(--border); background: var(--bg-panel-2);
  color: var(--text-dim); font-size: 11px; font-weight: 600; letter-spacing: .5px;
  padding: 4px 8px; border-radius: 6px; cursor: pointer;
  transition: all .15s;
}
.btn-lang:hover { color: var(--accent); border-color: var(--accent-border); }

.btn-mini {
  margin-left: auto;
  border: 1px solid var(--border); background: transparent;
  color: var(--text-dim); font-size: 11px; font-family: 'Segoe UI', 'Microsoft YaHei', sans-serif;
  padding: 3px 10px; border-radius: 5px; cursor: pointer;
}
.btn-mini:hover { color: var(--accent); border-color: var(--accent-border); }

.link {
  border: none; background: none; color: var(--accent);
  font-size: 11.5px; cursor: pointer; padding: 0;
}
.link:hover { text-decoration: underline; }

.export-zone { margin-top: auto; padding: 16px 18px 18px; }
.btn-export {
  width: 100%;
  display: flex; align-items: center; justify-content: center; gap: 8px;
  padding: 12px 0;
  border: none; border-radius: 9px;
  background: linear-gradient(135deg, #f5a524, #fb923c);
  color: #1a1407; font-size: 14.5px; font-weight: 600;
  cursor: pointer;
  box-shadow: 0 4px 18px rgba(245,165,36,.25);
  transition: transform .15s, box-shadow .15s, filter .15s;
}
.btn-export:hover:not(:disabled) { transform: translateY(-1px); box-shadow: 0 6px 24px rgba(245,165,36,.35); }
.btn-export:disabled { filter: grayscale(.7) brightness(.6); cursor: not-allowed; box-shadow: none; }

.spinner {
  width: 15px; height: 15px; border-radius: 50%;
  border: 2px solid rgba(26,20,7,.3); border-top-color: #1a1407;
  animation: spin .7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Toast */
.toast {
  position: fixed; bottom: 22px; left: 50%; transform: translateX(-50%);
  display: flex; align-items: center; gap: 8px;
  max-width: 82%;
  padding: 10px 14px; border-radius: 9px;
  font-size: 12.5px; z-index: 99;
  box-shadow: 0 10px 32px rgba(0,0,0,.5);
}
.toast.ok { background: #10231a; border: 1px solid rgba(74,222,128,.35); color: #86efac; }
.toast.err { background: #2a1518; border: 1px solid rgba(248,113,113,.35); color: #fca5a5; }
.toast span { word-break: break-all; }
.toast-close {
  border: none; background: none; color: inherit; opacity: .6;
  font-size: 15px; cursor: pointer; padding: 0 2px; flex-shrink: 0;
}
.toast-enter-active, .toast-leave-active { transition: all .25s ease; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(10px); }
</style>
