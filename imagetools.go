package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"

	"github.com/disintegration/imaging"
)

// ===================== 圆角自动识别 =====================

// pixelAt 读取 (x, y) 处的像素色
func pixelAt(img image.Image, bounds image.Rectangle, x, y int) color.RGBA {
	r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

// isBg 判断像素是否属于角部背景（透明或与背景色接近）
func isBg(c color.RGBA, bg color.RGBA) bool {
	return c.A < 128 || colorDistance(c, bg) <= 60
}

// DetectCornerRadius 自动检测图片四角是否有多余背景（圆角外区域），
// 返回建议的切割圆角半径（原图像素）和是否检测到。
// 原理：
//  1. 角部背景色需与中心内容色差异明显；
//  2. 沿对角线扫描找到"背景 -> 主图"转折点 t（t = pad + r·0.293）；
//  3. 沿边缘逐行扫描，最大转折距离 e = pad + r；
//  4. 联立解出 r = (e - t) / 0.707，兼容图标四周有留白的情况。
func DetectCornerRadius(img image.Image) (radius int, detected bool) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 40 || h < 40 {
		return 0, false
	}

	// 中心内容色参考
	content := averageColor(img, w/2-4, h/2-4, 8, 8)

	// 四个角：{角点x, 角点y, 向内方向dx, 向内方向dy}
	corners := [4][4]int{
		{0, 0, 1, 1}, {w - 1, 0, -1, 1}, {0, h - 1, 1, -1}, {w - 1, h - 1, -1, -1},
	}

	radii := []int{}
	for _, c := range corners {
		cx, cy, dx, dy := c[0], c[1], c[2], c[3]

		// 角部背景色（角内 1~3px 采样）
		bg := averageColor(img, cx+dx, cy+dy, 3, 3)

		// 背景与中心内容差异必须明显，否则该角没有可切背景
		if colorDistance(bg, content) < 30 {
			continue
		}

		// 对角线扫描：转折点 t
		t, ok := scanCornerDiagonal(img, bounds, cx, cy, dx, dy, bg)
		if !ok || t < 3 {
			continue
		}

		// 角部背景均匀性校验（排除照片/渐变背景误判）
		if !cornerBgUniform(img, bounds, cx, cy, dx, dy, bg, t) {
			continue
		}

		// 边缘扫描：最大转折 e = pad + r
		e, ok2 := scanCornerEdges(img, bounds, cx, cy, dx, dy, bg)
		if !ok2 || e <= t {
			continue
		}

		r := int(float64(e-t)/0.7071) + 1
		if r > 0 {
			// 用最小二乘圆弧拟合精化半径（更稳更准），结果合理时采用
			if rf, okf := fitCornerArc(img, bounds, cx, cy, dx, dy, bg, t, r); okf && rf >= r/2 && rf <= r*3/2 {
				r = rf
			}
			radii = append(radii, r)
		}
	}
	if len(radii) == 0 {
		return 0, false
	}

	// 取中位数 + 10% 余量（把抗锯齿边缘也切干净）
	sort.Ints(radii)
	median := radii[len(radii)/2]
	result := median + median/10
	maxR := int(math.Min(float64(w), float64(h))) / 2
	if result > maxR {
		result = maxR
	}
	if result < 4 {
		result = 4
	}
	return result, true
}

// scanCornerDiagonal 从角点沿 45° 对角线向内扫描，返回转折点的轴向距离 t
func scanCornerDiagonal(img image.Image, bounds image.Rectangle, cx, cy, dx, dy int, bg color.RGBA) (int, bool) {
	w := bounds.Dx()
	h := bounds.Dy()
	limit := int(math.Min(float64(w), float64(h))) / 3
	if limit > 200 {
		limit = 200
	}
	for step := 1; step < limit; step++ {
		x := cx + dx*step
		y := cy + dy*step
		if x < 0 || x >= w || y < 0 || y >= h {
			return 0, false
		}
		if !isBg(pixelAt(img, bounds, x, y), bg) {
			// 抗噪：后续连续 2 点仍非背景才判定为转折，避免孤立噪点误判
			ok := true
			for k := 1; k <= 2; k++ {
				nx := cx + dx*(step+k)
				ny := cy + dy*(step+k)
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					break
				}
				if isBg(pixelAt(img, bounds, nx, ny), bg) {
					ok = false
					break
				}
			}
			if ok {
				return step, true
			}
			// 视为噪点，继续向内扫描
		}
	}
	return 0, false
}

// cornerBgUniform 校验角部背景区域颜色均匀（真实纯色背景），排除渐变/照片
func cornerBgUniform(img image.Image, bounds image.Rectangle, cx, cy, dx, dy int, bg color.RGBA, t int) bool {
	w := bounds.Dx()
	h := bounds.Dy()
	// 采样点都限制在背景区域内（对角线转折点 t 以内）
	pts := [][2]int{
		{t / 2, t / 2},
		{t * 9 / 10, 3},
		{3, t * 9 / 10},
		{t * 3 / 5, t * 3 / 5},
	}
	for _, p := range pts {
		x := cx + dx*p[0]
		y := cy + dy*p[1]
		if x < 0 || x >= w || y < 0 || y >= h {
			continue
		}
		if colorDistance(pixelAt(img, bounds, x, y), bg) > 15 {
			return false
		}
	}
	return true
}

// scanCornerEdges 沿角部两条边逐行/列扫描，返回最大转折距离 e（= pad + r）
func scanCornerEdges(img image.Image, bounds image.Rectangle, cx, cy, dx, dy int, bg color.RGBA) (int, bool) {
	w := bounds.Dx()
	h := bounds.Dy()
	rowLimit := h / 3
	if rowLimit > 200 {
		rowLimit = 200
	}
	colLimit := w / 3
	if colLimit > 200 {
		colLimit = 200
	}
	best := 0
	found := false
	for row := 0; row < rowLimit; row++ {
		y := cy + dy*row
		if y < 0 || y >= h {
			break
		}
		for col := 0; col < colLimit; col++ {
			x := cx + dx*col
			if x < 0 || x >= w {
				break
			}
			if !isBg(pixelAt(img, bounds, x, y), bg) {
				if col > best {
					best = col
				}
				found = true
				break
			}
		}
	}
	return best, found
}

// fitCornerArc 沿圆角弧段采样一组色块边界点，用最小二乘拟合圆角半径 r，
// 比单点估算（对角线+边缘）更稳更准。仅采集 v∈[t, t+0.72r] 的纯弧上样本，避开 pad 直线段。
func fitCornerArc(img image.Image, bounds image.Rectangle, cx, cy, dx, dy int, bg color.RGBA, t, rInit int) (int, bool) {
	w := bounds.Dx()
	h := bounds.Dy()
	if t < 2 || rInit <= 0 {
		return 0, false
	}
	vStart := t
	vEnd := t + int(float64(rInit)*0.72) // 弧止于 pad+r ≈ t + 0.7071*r
	if vEnd >= h {
		vEnd = h - 1
	}
	if vStart >= vEnd {
		return 0, false
	}

	var xs, ys []float64
	for v := vStart; v <= vEnd; v++ {
		py := cy + dy*v
		if py < 0 || py >= h {
			break
		}
		u := 0
		for u < w {
			px := cx + dx*u
			if px < 0 || px >= w {
				break
			}
			if !isBg(pixelAt(img, bounds, px, py), bg) {
				break
			}
			u++
		}
		if u < 3 {
			continue
		}
		xs = append(xs, float64(u))
		ys = append(ys, float64(v))
	}
	if len(xs) < 4 {
		return 0, false
	}

	// 圆弧方程 (x-C)²+(y-C)²=r²（圆心在 (C,C)，C=pad+r）
	// 展开： x²+y² = 2C(x+y) + (r²-2C²)
	// 对样本做线性最小二乘：Y=x²+y², X=x+y, Y=slope·X+intercept，slope=2C, intercept=r²-2C²
	n := float64(len(xs))
	var sx, sy, sxx, sxy float64
	for i := range xs {
		X := xs[i] + ys[i]
		Y := xs[i]*xs[i] + ys[i]*ys[i]
		sx += X
		sy += Y
		sxx += X * X
		sxy += X * Y
	}
	denom := n*sxx - sx*sx
	if math.Abs(denom) < 1e-3 {
		return 0, false
	}
	slope := (n*sxy - sx*sy) / denom // = 2C
	intercept := (sy - slope*sx) / n // = r²-2C²
	C := slope / 2
	r2 := intercept + 2*C*C
	if C <= 0 || r2 <= 0.5 {
		return 0, false
	}
	r := math.Sqrt(r2)
	// 物理合理性：pad=C-r 不应显著为负；半径不应越界
	if C-r < -3 || r < 1 || r > float64(t+rInit) {
		return 0, false
	}
	return int(r + 0.5), true
}

// averageColor 采样一个区域的平均色
func averageColor(img image.Image, x, y, w, h int) color.RGBA {
	var sr, sg, sb, sa, n uint64
	bounds := img.Bounds()
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if xx < bounds.Min.X || xx >= bounds.Max.X || yy < bounds.Min.Y || yy >= bounds.Max.Y {
				continue
			}
			r, g, b, a := img.At(xx, yy).RGBA()
			sr += uint64(r >> 8)
			sg += uint64(g >> 8)
			sb += uint64(b >> 8)
			sa += uint64(a >> 8)
			n++
		}
	}
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: uint8(sa / n)}
}

// colorDistance 两颜色的欧氏距离（0~441），双透明视为相同
func colorDistance(a, b color.RGBA) int {
	if a.A < 20 && b.A < 20 {
		return 0
	}
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return int(math.Sqrt(float64(dr*dr + dg*dg + db*db)))
}

// ===================== 圆角切割 =====================

// ApplyRoundedCorners 用圆角矩形蒙版切除四角（变透明），边缘做抗锯齿过渡
func ApplyRoundedCorners(img image.Image, radius int) *image.NRGBA {
	dst := imaging.Clone(img)
	if radius <= 0 {
		return dst
	}
	w := dst.Rect.Dx()
	h := dst.Rect.Dy()
	r := float64(radius)
	if r > float64(w)/2 {
		r = float64(w) / 2
	}
	if r > float64(h)/2 {
		r = float64(h) / 2
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := roundedRectSDF(float64(x)+0.5, float64(y)+0.5, float64(w), float64(h), r)
			if d <= -0.5 {
				continue // 完全在内部，保留
			}
			alpha := 0.5 - d // d=-0.5 -> 1.0（内）；d=0.5 -> 0.0（外）
			if alpha < 0 {
				alpha = 0
			}
			if alpha > 1 {
				alpha = 1
			}
			px := dst.NRGBAAt(x, y)
			px.A = uint8(float64(px.A) * alpha)
			dst.SetNRGBA(x, y, px)
		}
	}
	return dst
}

// roundedRectSDF 圆角矩形的带符号距离场（负=内部，正=外部）
func roundedRectSDF(px, py, w, h, r float64) float64 {
	hw, hh := w/2, h/2
	qx := math.Abs(px-hw) - (hw - r)
	qy := math.Abs(py-hh) - (hh - r)
	ox := math.Max(qx, 0)
	oy := math.Max(qy, 0)
	return math.Sqrt(ox*ox+oy*oy) + math.Min(math.Max(qx, qy), 0) - r
}

// ===================== 正方形裁剪与缩放 =====================

// CropSquare 按裁剪框（原图坐标）裁出正方形区域
func CropSquare(img image.Image, x, y, size int) *image.NRGBA {
	return imaging.Crop(img, image.Rect(x, y, x+size, y+size))
}

// ResizeHighQuality 高质量缩放（Lanczos）
func ResizeHighQuality(img image.Image, size int) *image.NRGBA {
	return imaging.Resize(img, size, size, imaging.Lanczos)
}

// ===================== ICO 编码 =====================

// EncodeICO 将多张图打包成单个 ICO 文件。
// 小尺寸（<=128）用 BMP 格式条目（ICO 原始标准，所有查看器都支持），
// 256 用 PNG 压缩（官方为大图标设计，体积小）。
func EncodeICO(images []image.Image) ([]byte, error) {
	var buf bytes.Buffer

	count := len(images)
	// ICO 文件头（6 字节）：保留0 + 类型1(ico) + 数量
	buf.Write([]byte{0, 0, 1, 0, byte(count), byte(count >> 8)})

	offset := 6 + 16*count
	var entryData [][]byte
	for _, img := range images {
		var data []byte
		var err error
		if img.Bounds().Dx() >= 256 {
			var pngBuf bytes.Buffer
			if err = png.Encode(&pngBuf, img); err != nil {
				return nil, err
			}
			data = pngBuf.Bytes()
		} else {
			data, err = encodeICOEntryBMP(img)
			if err != nil {
				return nil, err
			}
		}
		entryData = append(entryData, data)

		b := img.Bounds().Dx()
		bW := byte(b)
		if b >= 256 {
			bW = 0 // ICO 规范：256 用 0 表示
		}
		buf.WriteByte(bW) // 宽
		buf.WriteByte(bW) // 高
		buf.WriteByte(0)  // 调色板数
		buf.WriteByte(0)  // 保留
		buf.WriteByte(1)  // 色彩平面数
		buf.WriteByte(0)
		buf.WriteByte(32) // 位深
		buf.WriteByte(0)
		writeLE32(&buf, len(data)) // 数据长度
		writeLE32(&buf, offset)    // 数据偏移
		offset += len(data)
	}

	for _, data := range entryData {
		buf.Write(data)
	}
	return buf.Bytes(), nil
}

// encodeICOEntryBMP 将图像编码为 ICO 内的 32bpp BMP 条目（BITMAPINFOHEADER + BGRA 像素 + AND 掩码）
func encodeICOEntryBMP(img image.Image) ([]byte, error) {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()

	// BITMAPINFOHEADER（40 字节），注意 biHeight = 2*h（XOR 图 + AND 掩码各占一份高度）
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(w))
	binary.LittleEndian.PutUint32(header[8:], uint32(h*2))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	// biCompression = BI_RGB(0)，其余字段 0

	data := make([]byte, 0, 40+w*h*4+((w+7)/8+3)/4*4*h)
	data = append(data, header...)

	// 像素数据：BGRA，自底向上
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.At(b.Min.X+x, b.Min.Y+y)
			r, g, bl, a := c.RGBA()
			data = append(data, byte(bl>>8), byte(g>>8), byte(r>>8), byte(a>>8))
		}
	}

	// AND 掩码：每行 (w+7)/8 字节并按 4 字节对齐，32bpp 有 alpha 时全 0 即可
	rowBytes := (w + 7) / 8
	if rem := rowBytes % 4; rem != 0 {
		rowBytes += 4 - rem
	}
	data = append(data, make([]byte, rowBytes*h)...)

	return data, nil
}

func writeLE32(buf *bytes.Buffer, v int) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}

// ===================== 处理管线 =====================

// BuildIconPipeline 完整处理管线：正方形裁剪 -> 圆角切割 -> 各尺寸缩放 -> ICO 编码
func BuildIconPipeline(src image.Image, cropX, cropY, cropSize, cornerRadius int, sizes []int) ([]byte, error) {
	// 裁剪框越界保护
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if cropSize <= 0 || cropSize > w || cropSize > h {
		cropSize = int(math.Min(float64(w), float64(h)))
		cropX = (w - cropSize) / 2
		cropY = (h - cropSize) / 2
	}
	if cropX < 0 {
		cropX = 0
	}
	if cropY < 0 {
		cropY = 0
	}
	if cropX+cropSize > w {
		cropX = w - cropSize
	}
	if cropY+cropSize > h {
		cropY = h - cropSize
	}

	square := CropSquare(src, cropX, cropY, cropSize)
	rounded := ApplyRoundedCorners(square, cornerRadius)

	var iconImages []image.Image
	for _, s := range sizes {
		resized := ResizeHighQuality(rounded, s)
		// 小尺寸下圆角边缘容易残留杂边，缩放后再按比例重切一次
		if cornerRadius > 0 && s <= 48 {
			r := cornerRadius * s / cropSize
			if r > 0 {
				resized = ApplyRoundedCorners(resized, r)
			}
		}
		iconImages = append(iconImages, resized)
	}
	return EncodeICO(iconImages)
}
