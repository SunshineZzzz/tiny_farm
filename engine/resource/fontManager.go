package resource

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

// 标识一个已加载字体实例
//
// 同一字体资源可以按不同像素字号分别缓存
type FontKey struct {
	// 字体资源语义 key
	Key ResourceKey
	// 字体像素字号
	PixelSize int
}

// 单个字体资源实例
//
// 当前阶段只缓存字体文件和字号的生命周期信息，glyph 缓存与 atlas 后续阶段补齐
type Font struct {
	// 字体资源语义 key
	key ResourceKey
	// 字体像素字号
	pixelSize int
	// 字体来源路径
	sourcePath string
	// 原始字体文件数据
	data []byte
}

// 返回字体资源语义 key
func (f *Font) Key() ResourceKey {
	if f == nil {
		return ""
	}
	return f.key
}

// 返回字体像素字号
func (f *Font) PixelSize() int {
	if f == nil {
		return 0
	}
	return f.pixelSize
}

// 返回字体来源路径
func (f *Font) SourcePath() string {
	if f == nil {
		return ""
	}
	return f.sourcePath
}

// 管理字体资源的加载、缓存和释放
//
// 当前阶段只负责字体文件级缓存，glyph 缓存和 atlas 纹理在后续阶段接入
type fontManager struct {
	// 按字体 key 和字号缓存字体实例
	fonts map[FontKey]*Font
}

// 创建字体资源管理器
func newFontManager() *fontManager {
	return &fontManager{
		fonts: make(map[FontKey]*Font),
	}
}

// 加载字体并按 key 与字号缓存
//
// 如果同一 key 和字号已经加载，直接返回缓存实例
func (m *fontManager) loadFont(key ResourceKey, path string, pixelSize int) (*Font, error) {
	if m == nil {
		return nil, errors.New("font manager is nil")
	}
	if key == "" {
		return nil, errors.New("font key is empty")
	}
	if pixelSize <= 0 {
		return nil, errors.New("font pixel size is invalid")
	}
	if path == "" {
		return nil, errors.New("font path is empty")
	}

	fontKey := FontKey{Key: key, PixelSize: pixelSize}
	if font, ok := m.fonts[fontKey]; ok && font != nil {
		return font, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load font %q from %q: %w", key, path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("load font %q from %q: font file is empty", key, path)
	}

	font := &Font{
		key:        key,
		pixelSize:  pixelSize,
		sourcePath: path,
		data:       data,
	}
	m.fonts[fontKey] = font
	return font, nil
}

// 卸载指定字体实例
func (m *fontManager) unloadFont(key ResourceKey, pixelSize int) {
	if m == nil {
		return
	}
	delete(m.fonts, FontKey{Key: key, PixelSize: pixelSize})
}

// 清空全部字体缓存
func (m *fontManager) clearFonts() {
	if m == nil {
		return
	}
	m.fonts = make(map[FontKey]*Font)
}

// 返回按 key 和字号排序的字体调试信息
func (m *fontManager) fontDebugInfo() []FontDebugInfo {
	if m == nil || len(m.fonts) == 0 {
		return nil
	}

	keys := make([]FontKey, 0, len(m.fonts))
	for key, font := range m.fonts {
		if font == nil {
			continue
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b FontKey) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return a.PixelSize - b.PixelSize
	})

	info := make([]FontDebugInfo, 0, len(keys))
	for _, key := range keys {
		font := m.fonts[key]
		info = append(info, FontDebugInfo{
			Key:            font.key,
			SourcePath:     font.sourcePath,
			PixelSize:      font.pixelSize,
			GlyphCount:     0,
			AtlasPageCount: 0,
			MemoryBytes:    len(font.data),
		})
	}
	return info
}
