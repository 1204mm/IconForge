package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "golang.org/x/image/bmp"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ImageInfo 前端拿到的图片信息
type ImageInfo struct {
	Name           string `json:"name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	DataURL        string `json:"dataUrl"`
	DetectedRadius int    `json:"detectedRadius"`  // 自动识别的圆角半径
	CornerDetected bool   `json:"cornerDetected"`  // 是否识别到圆角外背景
	ContentX       int    `json:"contentX"`        // 自动定位的内容包围盒 x
	ContentY       int    `json:"contentY"`        // 自动定位的内容包围盒 y
	ContentW       int    `json:"contentW"`        // 自动定位的内容包围盒 w
	ContentH       int    `json:"contentH"`        // 自动定位的内容包围盒 h
	ContentDetected bool  `json:"contentDetected"` // 是否自动定位到内容（非铺满整图）
}

// ExportParams 导出 ICO 的参数
type ExportParams struct {
	ImageData    string `json:"imageData"`    // 原图 dataURL
	CropX        int    `json:"cropX"`        // 裁剪框（原图坐标）
	CropY        int    `json:"cropY"`
	CropSize     int    `json:"cropSize"`
	CornerRadius int    `json:"cornerRadius"` // 圆角半径，0=不切
	Sizes        []int  `json:"sizes"`        // ICO 内包含的尺寸
}

// App 应用实例
type App struct {
	ctx context.Context
}

// NewApp 创建应用
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// loadImageByPath 读取并解析图片文件
func loadImageByPath(path string) (*ImageInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("不支持的图片格式（支持 PNG / JPG / BMP / GIF）: %v", err)
	}
	bounds := img.Bounds()
	mime := "image/" + format
	if format == "jpg" {
		mime = "image/jpeg"
	}

	// 自动识别圆角 + 内容包围盒（组合检测：自动裁剪生效时半径在裁剪区域上重测，保证尺度一致）
	radius, detected, cx, cy, cw, ch, contentOK := DetectIconParams(img)

	return &ImageInfo{
		Name:            filepath.Base(path),
		Width:           bounds.Dx(),
		Height:          bounds.Dy(),
		DataURL:         "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
		DetectedRadius:  radius,
		CornerDetected:  detected,
		ContentX:        cx,
		ContentY:        cy,
		ContentW:        cw,
		ContentH:        ch,
		ContentDetected: contentOK,
	}, nil
}

// OpenImageDialog 弹出文件选择框选择图片，用户取消时返回 null
func (a *App) OpenImageDialog() (*ImageInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择图片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片文件 (*.png;*.jpg;*.jpeg;*.bmp;*.gif)", Pattern: "*.png;*.jpg;*.jpeg;*.bmp;*.gif"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return loadImageByPath(path)
}

// LoadImageByPath 按路径加载图片（拖拽场景）
func (a *App) LoadImageByPath(path string) (*ImageInfo, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".bmp", ".gif":
	default:
		return nil, fmt.Errorf("不支持的文件类型: %s", ext)
	}
	return loadImageByPath(path)
}

// ExportIcons 导出图标 ZIP：每个尺寸一个独立 ICO + 一个多尺寸合一的 icon.ico，打包为 zip
// 返回保存路径（用户取消返回空）
func (a *App) ExportIcons(p ExportParams) (string, error) {
	if len(p.Sizes) == 0 {
		return "", fmt.Errorf("请至少选择一个 ICO 尺寸")
	}

	// 解码原图
	idx := strings.Index(p.ImageData, "base64,")
	if idx < 0 {
		return "", fmt.Errorf("图片数据无效")
	}
	raw, err := base64.StdEncoding.DecodeString(p.ImageData[idx+7:])
	if err != nil {
		return "", fmt.Errorf("图片数据解码失败: %v", err)
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("图片解码失败: %v", err)
	}

	// 尺寸从小到大排序，保证 zip 内文件顺序稳定
	sizes := append([]int{}, p.Sizes...)
	sort.Ints(sizes)

	// 打包：icon.ico（多尺寸合一，Windows 直接可用） + icon_<size>.ico（各尺寸独立文件）
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	addEntry := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}

	// 多尺寸合一
	combined, err := BuildIconPipeline(src, p.CropX, p.CropY, p.CropSize, p.CornerRadius, sizes)
	if err != nil {
		return "", fmt.Errorf("生成 ICO 失败: %v", err)
	}
	if err := addEntry("icon.ico", combined); err != nil {
		return "", fmt.Errorf("写入 ZIP 失败: %v", err)
	}

	// 各尺寸独立文件
	for _, s := range sizes {
		one, err := BuildIconPipeline(src, p.CropX, p.CropY, p.CropSize, p.CornerRadius, []int{s})
		if err != nil {
			return "", fmt.Errorf("生成 %dpx ICO 失败: %v", s, err)
		}
		if err := addEntry(fmt.Sprintf("icon_%d.ico", s), one); err != nil {
			return "", fmt.Errorf("写入 ZIP 失败: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("打包 ZIP 失败: %v", err)
	}

	// 保存对话框
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存图标压缩包",
		DefaultFilename: "icons.zip",
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP 压缩包 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", nil // 用户取消
	}
	if !strings.EqualFold(filepath.Ext(savePath), ".zip") {
		savePath += ".zip"
	}

	if err := os.WriteFile(savePath, zipBuf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}
	return savePath, nil
}
