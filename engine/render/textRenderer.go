package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils"
	"tiny_farm/engine/utils/defs"
	"tiny_farm/engine/utils/dispatch"
	"tiny_farm/engine/utils/event"
	"tiny_farm/engine/utils/lrucache"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
)

const (
	// 默认文本渲染配置路径
	defaultTextRenderConfigPath = "config/text_render.json"
	// 默认布局缓存容量
	defaultLayoutCacheCapacity = 100
	// 默认 UI 文本样式 key
	defaultUIStyleKey = "ui/default"
	// 默认世界文本样式 key
	defaultWorldStyleKey = "world/default"
)

// 文本基础排版方向
type textDirection int

const (
	// 从左到右
	textDirectionLeftToRight textDirection = iota
	// 从右到左
	textDirectionRightToLeft
	// 从上到下
	textDirectionTopToBottom
	// 从下到上
	textDirectionBottomToTop
)

// 保存一条文本样式配置
type TextStyleEntry struct {
	// 文字颜色或渐变参数
	Color defs.ColorOptions
	// 阴影参数
	Shadow *defs.ShadowOptions
	// 布局参数
	Layout *defs.LayoutOptions
}

// 文本布局缓存键，对应的value是textLayout
// 用来判断“这次请求能不能复用之前算好的布局”
type layoutKey struct {
	// 字体资源 key
	fontKey defs.ResourceKey
	// 字体像素字号
	pixelSize int
	// 原始文本内容
	text string
	// 字符额外间距，单位像素
	letterSpacing float32
	// 行距缩放
	lineSpacingScale float32
	// 字形水平缩放
	glyphScaleX float32
	// 字形垂直缩放
	glyphScaleY float32
}

// 单个 glyph 的布局结果
//
// rect 是相对文本起点的绘制矩形，真正绘制时再叠加 position
type glyphPlacement struct {
	// 字形级别信息
	// glyph 缓存条目，同一个 glyph 每次都一样
	glyph abstract.IFontGlyph
	// 布局级别信息
	// 相对文本起点的绘制矩形
	// 表示这一次排版后，这个 glyph 在这段文本里的绘制矩形
	rect mgl32.Vec4
	// uv 确实是冗余缓存
	// glyph 在 atlas 中的 UV 范围
	uv mgl32.Vec4
}

// 一段文本的布局结果
//
// 当前由 HarfBuzz shaping 后的 glyph index 序列生成
//
// 字符串 "Hello"
// -> HarfBuzz shaping
// -> 查询 glyph index + advance/offset
// -> 查询 glyph atlas
// -> 生成 glyphPlacement
// -> 保存成 textLayout
// -> 绘制
type textLayout struct {
	// 文本布局包围尺寸
	size mgl32.Vec2
	// 已排布的 glyph 列表
	glyphs []glyphPlacement
	// 文本行数
	lineCount int
}

// 对应默认样式 key 配置
type textDefaultStyleKeysConfig struct {
	// 默认 UI 文本样式 key
	UI string `json:"ui"`
	// 默认世界文本样式 key
	World string `json:"world"`
}

// 对应颜色配置
type textColorConfig struct {
	// 起始颜色，格式为 #RRGGBB 或 #RRGGBBAA
	StartColor string `json:"start_color"`
	// 结束颜色，格式为 #RRGGBB 或 #RRGGBBAA
	EndColor string `json:"end_color"`
	// 是否启用渐变
	UseGradient bool `json:"use_gradient"`
	// 渐变方向角度，单位弧度
	AngleRadians float32 `json:"angle_radians"`
	// 渐变方向角度，单位度
	AngleDegrees float32 `json:"angle_degrees"`
}

// 对应阴影配置
type textShadowConfig struct {
	// 是否启用阴影
	Enabled bool `json:"enabled"`
	// 阴影偏移，单位像素
	Offset [2]float32 `json:"offset"`
	// 阴影颜色，格式为 #RRGGBB 或 #RRGGBBAA
	Color string `json:"color"`
}

// 对应布局配置
type textLayoutConfig struct {
	// 字符额外间距，单位像素
	LetterSpacing float32 `json:"letter_spacing"`
	// 行距缩放
	LineSpacingScale float32 `json:"line_spacing_scale"`
	// 字形缩放
	GlyphScale [2]float32 `json:"glyph_scale"`
}

// 对应单条样式配置
type textStyleConfig struct {
	// 颜色或渐变配置
	Color textColorConfig `json:"color"`
	// 阴影配置
	Shadow *textShadowConfig `json:"shadow"`
	// 布局配置
	Layout *textLayoutConfig `json:"layout"`
}

// 对应 text_renderer 配置节点
type textRenderConfig struct {
	// 文本方向配置
	Direction string `json:"direction"`
	// 默认文本方向配置
	DefaultDirection string `json:"default_direction"`
	// 文本语言标签, 用于 HarfBuzz 排版
	Language string `json:"language"`
	// HarfBuzz feature 配置
	// "features": ["kern=1", "liga=1", "clig=1"]
	// kern=1：开启字偶距
	// liga=1：开启标准连字，比如 fi
	// clig=1：开启上下文连字
	Features []string `json:"features"`
	// 布局缓存容量
	LayoutCacheCapacity *int `json:"layout_cache_capacity"`
	// 默认样式 key 配置
	DefaultStyleKeys textDefaultStyleKeysConfig `json:"default_style_keys"`
	// 文本样式表
	Styles map[string]textStyleConfig `json:"styles"`
}

// 对应 text_render.json 根节点
type textRenderConfigFile struct {
	// 文本渲染器配置节点
	TextRenderer textRenderConfig `json:"text_renderer"`
}

// 描述文本渲染器当前缓存和样式状态
//
// 供日志、测试和 Debug UI 读取，不暴露具体布局缓存内容
type TextRendererDebugInfo struct {
	// 布局缓存容量上限
	LayoutCacheCapacity int
	// 当前布局缓存条目数
	LayoutCacheSize int
	// 样式或布局配置变更版本
	LayoutRevision uint64
	// 按字典序排列的文本样式 key
	TextStyleKeys []string
	// 默认 UI 文本样式 key
	DefaultUIStyleKey string
	// 默认世界文本样式 key
	DefaultWorldStyleKey string
}

// 负责把字体 glyph 串成可测量和可绘制文本
//
// 当前复用 FontManager 的 glyph index 级 glyph cache，并调用 Renderer 绘制 atlas 子区域
type TextRenderer struct {
	// 用于查询已加载字体资源
	resourceManager abstract.IResourceManager
	// 用于提交 glyph 贴图绘制命令
	renderer *Renderer
	// 已计算文本布局缓存
	layoutCache *lrucache.LRUCache[layoutKey, textLayout]
	// 布局缓存最大条目数
	layoutCacheCapacity int
	// 字体生命周期事件连接
	eventConnections []dispatch.Connection
	// 已加载文本样式
	styles map[string]TextStyleEntry
	// 默认 UI 样式 key
	defaultUIStyleKey string
	// 默认世界文本样式 key
	defaultWorldStyleKey string
	// 默认文本方向
	defaultDirection textDirection
	// 默认语言标签
	defaultLanguageTag string
	// 当前启用的 HarfBuzz feature
	activeFeatures []harfbuzz.Feature
	// 样式或布局配置变更版本
	// 让依赖文本尺寸/排版结果的外部系统知道：之前算过的东西可能过期了
	layoutRevision uint64
}

// 确保 TextRenderer 实现 ITextRenderer 接口
var _ abstract.ITextRenderer = (*TextRenderer)(nil)

// 创建文本渲染器
func NewTextRenderer(resourceManager abstract.IResourceManager, renderer *Renderer, dispatcher *dispatch.Dispatcher) (*TextRenderer, error) {
	if resourceManager == nil {
		return nil, errors.New("text renderer resource manager is nil")
	}
	if renderer == nil {
		return nil, errors.New("text renderer renderer is nil")
	}
	textRenderer := &TextRenderer{
		resourceManager:      resourceManager,
		renderer:             renderer,
		layoutCache:          lrucache.NewLRUCache[layoutKey, textLayout](defaultLayoutCacheCapacity),
		layoutCacheCapacity:  defaultLayoutCacheCapacity,
		styles:               make(map[string]TextStyleEntry),
		defaultUIStyleKey:    defaultUIStyleKey,
		defaultWorldStyleKey: defaultWorldStyleKey,
		defaultDirection:     textDirectionLeftToRight,
		defaultLanguageTag:   "zh-Hans",
	}
	textRenderer.ensureBuiltinTextStyles()
	if err := textRenderer.loadConfig(defaultTextRenderConfigPath); err != nil {
		return nil, err
	}
	if dispatcher != nil {
		textRenderer.connectFontEvents(dispatcher)
	}
	return textRenderer, nil
}

// 测量已加载字体下的文本包围尺寸
func (t *TextRenderer) MeasureText(text string, fontKey defs.ResourceKey, pixelSize int, otherArgs ...abstract.ITextArg) (mgl32.Vec2, error) {
	fontPath := string(fontKey)
	var styleKey defs.TextStyleKey
	var params *defs.TextRenderParams
	var overrides *defs.TextRenderOverrides
	var options *defs.LayoutOptions
	styleProvided := false
	layoutProvided := false
	for _, arg := range otherArgs {
		switch value := arg.(type) {
		case defs.FontPath:
			fontPath = string(value)
		case defs.TextStyleKey:
			styleKey = value
			styleProvided = true
		case *defs.LayoutOptions:
			options = value
			layoutProvided = true
		case *defs.TextRenderParams:
			params = value
			if value != nil {
				options = value.Layout
				layoutProvided = value.Layout != nil
			}
		case *defs.TextRenderOverrides:
			overrides = value
		}
	}
	layoutOptions := t.resolveMeasureLayoutOptions(styleKey, params, overrides, options, styleProvided, layoutProvided)
	layout, err := t.layoutText(text, fontKey, pixelSize, fontPath, layoutOptions)
	if err != nil {
		return mgl32.Vec2{}, err
	}
	return layout.size, nil
}

// 绘制 UI 逻辑坐标系文本
func (t *TextRenderer) DrawUIText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, otherArgs ...abstract.ITextArg) error {
	fontPath := string(fontKey)
	var styleKey defs.TextStyleKey
	var params *defs.TextRenderParams
	var overrides *defs.TextRenderOverrides
	styleKeyProvided := false
	paramsProvided := false
	for _, arg := range otherArgs {
		switch value := arg.(type) {
		case defs.FontPath:
			fontPath = string(value)
		case defs.TextStyleKey:
			styleKey = value
			styleKeyProvided = true
		case *defs.TextRenderParams:
			params = value
			paramsProvided = true
		case *defs.LayoutOptions:
			params = &defs.TextRenderParams{Layout: value}
			paramsProvided = true
		case *defs.TextRenderOverrides:
			overrides = value
		}
	}
	return t.drawText(text, fontKey, pixelSize, position, fontPath,
		t.resolveTextRenderParams(styleKey, params, overrides, true,
			styleKeyProvided, paramsProvided), true)
}

// 绘制世界坐标系文本
func (t *TextRenderer) DrawWorldText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, otherArgs ...abstract.ITextArg) error {
	fontPath := string(fontKey)
	var styleKey defs.TextStyleKey
	var params *defs.TextRenderParams
	var overrides *defs.TextRenderOverrides
	styleKeyProvided := false
	paramsProvided := false
	for _, arg := range otherArgs {
		switch value := arg.(type) {
		case defs.FontPath:
			fontPath = string(value)
		case defs.TextStyleKey:
			styleKey = value
			styleKeyProvided = true
		case *defs.TextRenderParams:
			params = value
			paramsProvided = true
		case *defs.LayoutOptions:
			params = &defs.TextRenderParams{Layout: value}
			paramsProvided = true
		case *defs.TextRenderOverrides:
			overrides = value
		}
	}
	return t.drawText(text, fontKey, pixelSize, position, fontPath,
		t.resolveTextRenderParams(styleKey, params, overrides, false,
			styleKeyProvided, paramsProvided), false)
}

// 绘制一段文本，可按参数决定使用 UI 坐标或世界坐标
//
// 当前先绘制阴影再绘制正文，阴影复用同一套 glyph 布局
func (t *TextRenderer) drawText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, fontPath string, params *defs.TextRenderParams, ui bool) error {
	if text == "" {
		return nil
	}
	if t == nil || t.renderer == nil {
		return errors.New("text renderer renderer is nil")
	}
	layout, err := t.layoutText(text, fontKey, pixelSize, fontPath, params.Layout)
	if err != nil {
		return err
	}
	if params.Shadow != nil && params.Shadow.Enabled {
		shadowPosition := position.Add(params.Shadow.Offset)
		if err := t.drawTextLayout(layout, shadowPosition, defs.SolidTextColorOptions(params.Shadow.Color), ui); err != nil {
			return err
		}
	}
	return t.drawTextLayout(layout, position, resolvedTextColorOptions(params), ui)
}

// 提交已经布局好的 glyph 绘制命令
//
// layout 中的矩形是相对文本起点的坐标，这里叠加 position 生成最终绘制位置
func (t *TextRenderer) drawTextLayout(layout *textLayout, position mgl32.Vec2, color defs.ColorOptions, ui bool) error {
	if layout == nil {
		return nil
	}
	for _, placement := range layout.glyphs {
		dstRect := placement.rect
		dstRect[0] += position.X()
		dstRect[1] += position.Y()
		texture, ok := placement.glyph.GlyphTexture().(*Texture)
		if !ok || texture == nil || dstRect.Z() <= 0 || dstRect.W() <= 0 {
			continue
		}
		var err error
		if ui {
			err = t.renderer.DrawUITextureColorOptions(texture, dstRect, placement.uv, color)
		} else {
			err = t.renderer.DrawWorldTextureColorOptions(texture, dstRect, placement.uv, color)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// 获取文本布局，优先复用缓存
//
// 缓存键包含字体、字号、文本和全部布局参数
func (t *TextRenderer) layoutText(text string, fontKey defs.ResourceKey, pixelSize int, fontPath string, options *defs.LayoutOptions) (*textLayout, error) {
	if t == nil || t.resourceManager == nil {
		return nil, errors.New("text renderer resource manager is nil")
	}
	options = normalizeLayoutOptions(options)
	key := makeLayoutKey(text, fontKey, pixelSize, options)
	if t.layoutCache != nil {
		if layout := t.layoutCache.Get(key); layout != nil {
			return layout, nil
		}
	}
	font, err := t.resourceManager.LoadFont(fontKey, pixelSize, fontPath)
	if err != nil {
		return nil, err
	}
	layout, err := t.buildTextLayout(text, font, options)
	if err != nil {
		return nil, err
	}
	if t.layoutCacheCapacity > 0 {
		if t.layoutCache == nil {
			t.layoutCache = lrucache.NewLRUCache[layoutKey, textLayout](t.layoutCacheCapacity)
		}
		t.layoutCache.Put(key, layout)
	}
	return layout, nil
}

// 清空全部文本布局缓存
func (t *TextRenderer) ClearLayoutCache() {
	if t == nil || t.layoutCache == nil {
		return
	}
	t.layoutCache.Clear()
}

// 设置布局缓存容量
//
// 容量小于 0 时按 0 处理，0 表示不保留布局缓存
func (t *TextRenderer) SetLayoutCacheCapacity(capacity int) {
	if t == nil {
		return
	}
	if capacity < 0 {
		capacity = 0
	}
	t.layoutCacheCapacity = capacity
	if t.layoutCache == nil {
		t.layoutCache = lrucache.NewLRUCache[layoutKey, textLayout](capacity)
		return
	}
	t.layoutCache.SetCapacity(capacity)
}

// 返回当前布局缓存容量
func (t *TextRenderer) LayoutCacheCapacity() int {
	if t == nil {
		return 0
	}
	return t.layoutCacheCapacity
}

// 返回当前布局缓存条目数
func (t *TextRenderer) LayoutCacheSize() int {
	if t == nil {
		return 0
	}
	return t.layoutCache.Len()
}

// 返回文本渲染器调试信息
func (t *TextRenderer) DebugInfo() TextRendererDebugInfo {
	if t == nil {
		return TextRendererDebugInfo{}
	}
	return TextRendererDebugInfo{
		LayoutCacheCapacity:  t.layoutCacheCapacity,
		LayoutCacheSize:      t.LayoutCacheSize(),
		LayoutRevision:       t.layoutRevision,
		TextStyleKeys:        t.ListTextStyleKeys(),
		DefaultUIStyleKey:    t.defaultUIStyleKey,
		DefaultWorldStyleKey: t.defaultWorldStyleKey,
	}
}

// 释放文本渲染器持有的事件连接
func (t *TextRenderer) Close() {
	if t == nil {
		return
	}
	for _, connection := range t.eventConnections {
		connection.Release()
	}
	t.eventConnections = nil
	t.ClearLayoutCache()
}

// 连接字体生命周期事件
func (t *TextRenderer) connectFontEvents(dispatcher *dispatch.Dispatcher) {
	if t == nil || dispatcher == nil {
		return
	}
	t.eventConnections = append(t.eventConnections,
		dispatch.SinkOf[event.FontUnloadedEvent](dispatcher).Connect(t.onFontUnloaded),
		dispatch.SinkOf[event.FontsClearedEvent](dispatcher).Connect(t.onFontsCleared),
	)
}

// 单个字体卸载时移除对应布局缓存
func (t *TextRenderer) onFontUnloaded(fontEvent event.FontUnloadedEvent) {
	if t == nil || t.layoutCache == nil || t.layoutCache.Len() == 0 {
		return
	}
	t.layoutCache.DeleteFunc(func(key layoutKey, _ *textLayout) bool {
		return key.fontKey == fontEvent.Key && key.pixelSize == fontEvent.PixelSize
	})
}

// 全部字体清空时清空布局缓存
func (t *TextRenderer) onFontsCleared(event.FontsClearedEvent) {
	if t == nil {
		return
	}
	t.ClearLayoutCache()
}

// 设置文本样式
//
// 样式会被复制保存，修改样式后会递增布局版本并清空布局缓存
func (t *TextRenderer) SetTextStyle(key string, style TextStyleEntry) error {
	if t == nil {
		return errors.New("text renderer is nil")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("text style key is empty")
	}

	if t.styles == nil {
		t.styles = make(map[string]TextStyleEntry)
	}

	resolved := cloneTextStyleEntry(TextStyleEntry{
		Color:  normalizeTextColorOptions(style.Color),
		Shadow: style.Shadow,
		Layout: normalizeLayoutOptions(style.Layout),
	})

	previous, exists := t.styles[key]
	t.styles[key] = resolved

	// 新增样式不会影响已有文本布局
	if !exists {
		return nil
	}

	// 只修改颜色或阴影不会影响文本布局
	if layoutOptionsEqual(previous.Layout, resolved.Layout) {
		return nil
	}

	t.layoutRevision++
	t.ClearLayoutCache()
	return nil
}

// 比较规范化后的文本布局参数
func layoutOptionsEqual(left, right *defs.LayoutOptions) bool {
	resolvedLeft := normalizeLayoutOptions(left)
	resolvedRight := normalizeLayoutOptions(right)
	return *resolvedLeft == *resolvedRight
}

// 查询文本样式
func (t *TextRenderer) GetTextStyle(key string) (TextStyleEntry, bool) {
	if t == nil || t.styles == nil {
		return TextStyleEntry{}, false
	}
	style, ok := t.styles[key]
	if !ok {
		return TextStyleEntry{}, false
	}
	return cloneTextStyleEntry(style), true
}

// 判断文本样式是否存在
func (t *TextRenderer) HasTextStyle(key string) bool {
	if t == nil || t.styles == nil {
		return false
	}
	_, ok := t.styles[key]
	return ok
}

// 返回按字典序排列的文本样式 key
func (t *TextRenderer) ListTextStyleKeys(prefix ...string) []string {
	if t == nil || len(t.styles) == 0 {
		return nil
	}
	filter := ""
	if len(prefix) != 0 {
		filter = prefix[0]
	}
	keys := make([]string, 0, len(t.styles))
	for key := range t.styles {
		if filter == "" || strings.HasPrefix(key, filter) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// 返回默认 UI 文本样式 key
func (t *TextRenderer) DefaultUIStyleKey() string {
	if t == nil {
		return ""
	}
	return t.defaultUIStyleKey
}

// 返回默认世界文本样式 key
func (t *TextRenderer) DefaultWorldStyleKey() string {
	if t == nil {
		return ""
	}
	return t.defaultWorldStyleKey
}

// 返回样式或布局配置变更版本
func (t *TextRenderer) LayoutRevision() uint64 {
	if t == nil {
		return 0
	}
	return t.layoutRevision
}

// 加载文本渲染配置
//
// 配置文件不存在时保留内建默认值，配置存在但内容非法时返回错误
func (t *TextRenderer) loadConfig(configPath string) error {
	if t == nil {
		return errors.New("text renderer is nil")
	}
	if strings.TrimSpace(configPath) == "" {
		return nil
	}

	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load text render config %q: %w", configPath, err)
	}

	var configFile textRenderConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return fmt.Errorf("parse text render config %q: %w", configPath, err)
	}
	return t.applyTextRenderConfig(configFile.TextRenderer)
}

// 应用文本渲染配置
func (t *TextRenderer) applyTextRenderConfig(config textRenderConfig) error {
	if t == nil {
		return errors.New("text renderer is nil")
	}
	layoutChanged := false

	if config.LayoutCacheCapacity != nil {
		if *config.LayoutCacheCapacity < 0 {
			return errors.New("text renderer layout_cache_capacity is invalid")
		}
		if t.layoutCacheCapacity != *config.LayoutCacheCapacity {
			t.SetLayoutCacheCapacity(*config.LayoutCacheCapacity)
			layoutChanged = true
		}
	}

	directionText := config.Direction
	if directionText == "" {
		directionText = config.DefaultDirection
	}
	if strings.TrimSpace(directionText) != "" {
		direction, err := parseTextDirection(directionText, t.defaultDirection)
		if err != nil {
			return err
		}
		if direction != t.defaultDirection {
			t.defaultDirection = direction
			layoutChanged = true
		}
	}

	languageTag := config.Language
	if strings.TrimSpace(languageTag) != "" && languageTag != t.defaultLanguageTag {
		t.defaultLanguageTag = languageTag
		layoutChanged = true
	}

	if config.Features != nil {
		features, err := parseTextFeatures(config.Features)
		if err != nil {
			return err
		}
		if !sameTextFeatures(t.activeFeatures, features) {
			t.activeFeatures = features
			layoutChanged = true
		}
	}

	if config.DefaultStyleKeys.UI != "" {
		t.defaultUIStyleKey = config.DefaultStyleKeys.UI
	}
	if config.DefaultStyleKeys.World != "" {
		t.defaultWorldStyleKey = config.DefaultStyleKeys.World
	}

	for key, styleConfig := range config.Styles {
		if strings.TrimSpace(key) == "" {
			return errors.New("text renderer style key is empty")
		}
		style, err := parseTextStyleConfig(styleConfig)
		if err != nil {
			return fmt.Errorf("parse text style %q: %w", key, err)
		}
		t.styles[key] = style
		layoutChanged = true
	}

	if _, ok := t.styles[t.defaultUIStyleKey]; !ok {
		return fmt.Errorf("text renderer default ui style %q is not defined", t.defaultUIStyleKey)
	}
	if _, ok := t.styles[t.defaultWorldStyleKey]; !ok {
		return fmt.Errorf("text renderer default world style %q is not defined", t.defaultWorldStyleKey)
	}
	if layoutChanged {
		t.layoutRevision++
		t.ClearLayoutCache()
	}
	return nil
}

// 解析文本方向配置
func parseTextDirection(value string, fallback textDirection) (textDirection, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return fallback, nil
	case "ltr", "left_to_right", "left-to-right":
		return textDirectionLeftToRight, nil
	case "rtl", "right_to_left", "right-to-left":
		return textDirectionRightToLeft, nil
	case "ttb", "top_to_bottom", "top-to-bottom":
		return textDirectionTopToBottom, nil
	case "btt", "bottom_to_top", "bottom-to-top":
		return textDirectionBottomToTop, nil
	default:
		return fallback, fmt.Errorf("text renderer direction %q is invalid", value)
	}
}

// 解析 HarfBuzz feature 配置
func parseTextFeatures(values []string) ([]harfbuzz.Feature, error) {
	features := make([]harfbuzz.Feature, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		feature, err := harfbuzz.ParseFeature(value)
		if err != nil {
			return nil, fmt.Errorf("text renderer feature %q is invalid: %w", value, err)
		}
		features = append(features, feature)
	}
	return features, nil
}

// 比较 feature 配置是否相同
func sameTextFeatures(a, b []harfbuzz.Feature) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 建立内建默认文本样式
func (t *TextRenderer) ensureBuiltinTextStyles() {
	if t == nil {
		return
	}
	style := TextStyleEntry{
		Color: defs.SolidTextColorOptions(mgl32.Vec4{1.0, 1.0, 1.0, 1.0}),
		Shadow: &defs.ShadowOptions{
			Enabled: true,
			Offset:  mgl32.Vec2{1.0, 1.0},
			Color:   mgl32.Vec4{0.0, 0.0, 0.0, 1.0},
		},
		Layout: normalizeLayoutOptions(nil),
	}
	t.styles[defaultUIStyleKey] = cloneTextStyleEntry(style)
	t.styles[defaultWorldStyleKey] = cloneTextStyleEntry(style)
}

// 解析单条文本样式配置
func parseTextStyleConfig(config textStyleConfig) (TextStyleEntry, error) {
	color := defs.SolidTextColorOptions(mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
	if strings.TrimSpace(config.Color.StartColor) != "" {
		parsed, err := utils.ParseHexColor(config.Color.StartColor)
		if err != nil {
			return TextStyleEntry{}, err
		}
		color.StartColor = parsed
	}
	if strings.TrimSpace(config.Color.EndColor) != "" {
		parsed, err := utils.ParseHexColor(config.Color.EndColor)
		if err != nil {
			return TextStyleEntry{}, err
		}
		color.EndColor = parsed
	} else {
		color.EndColor = color.StartColor
	}
	color.UseGradient = config.Color.UseGradient
	color.AngleRadians = config.Color.AngleRadians
	if config.Color.AngleDegrees != 0 {
		color.AngleRadians = config.Color.AngleDegrees * float32(math.Pi) / 180.0
	}

	var shadow *defs.ShadowOptions
	if config.Shadow != nil {
		shadowColor := mgl32.Vec4{0.0, 0.0, 0.0, 1.0}
		if strings.TrimSpace(config.Shadow.Color) != "" {
			parsed, err := utils.ParseHexColor(config.Shadow.Color)
			if err != nil {
				return TextStyleEntry{}, err
			}
			shadowColor = parsed
		}
		shadow = &defs.ShadowOptions{
			Enabled: config.Shadow.Enabled,
			Offset:  mgl32.Vec2{config.Shadow.Offset[0], config.Shadow.Offset[1]},
			Color:   shadowColor,
		}
	}

	layout := normalizeLayoutOptions(nil)
	if config.Layout != nil {
		layout = normalizeLayoutOptions(&defs.LayoutOptions{
			LetterSpacing:    config.Layout.LetterSpacing,
			LineSpacingScale: config.Layout.LineSpacingScale,
			GlyphScale:       mgl32.Vec2{config.Layout.GlyphScale[0], config.Layout.GlyphScale[1]},
		})
	}

	return TextStyleEntry{
		Color:  color,
		Shadow: shadow,
		Layout: layout,
	}, nil
}

// 复制文本样式，避免调用方修改共享指针
func cloneTextStyleEntry(style TextStyleEntry) TextStyleEntry {
	cloned := TextStyleEntry{
		Color: style.Color,
	}
	if style.Shadow != nil {
		shadow := *style.Shadow
		cloned.Shadow = &shadow
	}
	if style.Layout != nil {
		layout := *style.Layout
		cloned.Layout = &layout
	}
	return cloned
}

// 解析测量使用的布局参数
func (t *TextRenderer) resolveMeasureLayoutOptions(styleKey defs.TextStyleKey, params *defs.TextRenderParams,
	overrides *defs.TextRenderOverrides, options *defs.LayoutOptions, styleProvided bool, layoutProvided bool) *defs.LayoutOptions {
	var resolved *defs.LayoutOptions
	if t != nil && styleProvided {
		if style, ok := t.styles[string(styleKey)]; ok {
			resolved = style.Layout
		}
	}
	if params != nil && params.Layout != nil {
		resolved = params.Layout
	}
	if layoutProvided && options != nil {
		resolved = options
	}
	if overrides != nil && overrides.Layout != nil {
		resolved = overrides.Layout
	}
	return normalizeLayoutOptions(resolved)
}

// 根据默认样式、显式样式和覆盖参数解析本次绘制参数
func (t *TextRenderer) resolveTextRenderParams(styleKey defs.TextStyleKey, params *defs.TextRenderParams, overrides *defs.TextRenderOverrides,
	ui bool, styleKeyProvided bool, paramsProvided bool) *defs.TextRenderParams {
	var resolved *defs.TextRenderParams
	if t != nil {
		key := ""
		if styleKeyProvided {
			key = string(styleKey)
		} else if !paramsProvided {
			key = t.defaultWorldStyleKey
			if ui {
				key = t.defaultUIStyleKey
			}
		}
		if key != "" {
			if style, ok := t.styles[key]; ok {
				resolved = &defs.TextRenderParams{
					Color:  style.Color,
					Shadow: style.Shadow,
					Layout: style.Layout,
				}
			} else {
				fallbackKey := t.defaultWorldStyleKey
				if ui {
					fallbackKey = t.defaultUIStyleKey
				}
				if style, ok := t.styles[fallbackKey]; ok {
					resolved = &defs.TextRenderParams{
						Color:  style.Color,
						Shadow: style.Shadow,
						Layout: style.Layout,
					}
				}
			}
		}
	}
	if resolved == nil {
		resolved = &defs.TextRenderParams{}
	}
	if params != nil {
		if params.Color != (defs.ColorOptions{}) {
			resolved.Color = params.Color
		}
		if params.Shadow != nil {
			resolved.Shadow = params.Shadow
		}
		if params.Layout != nil {
			resolved.Layout = params.Layout
		}
	}
	if overrides != nil {
		if overrides.Color != nil {
			resolved.Color = *overrides.Color
		}
		if overrides.Shadow != nil {
			resolved.Shadow = overrides.Shadow
		}
		if overrides.Layout != nil {
			resolved.Layout = overrides.Layout
		}
	}
	return normalizeTextRenderParams(resolved)
}

// 创建布局缓存键
func makeLayoutKey(text string, fontKey defs.ResourceKey, pixelSize int, options *defs.LayoutOptions) layoutKey {
	return layoutKey{
		fontKey:          fontKey,
		pixelSize:        pixelSize,
		text:             text,
		letterSpacing:    options.LetterSpacing,
		lineSpacingScale: options.LineSpacingScale,
		glyphScaleX:      options.GlyphScale.X(),
		glyphScaleY:      options.GlyphScale.Y(),
	}
}

// 使用指定字体生成文本布局
//
// 当前通过 HarfBuzz 把每行文本转换成 glyph index 序列，再按 glyph 度量生成绘制矩形
func (t *TextRenderer) buildTextLayout(text string, font abstract.IFont, options *defs.LayoutOptions) (*textLayout, error) {
	if font == nil {
		return nil, errors.New("font is nil")
	}
	options = normalizeLayoutOptions(options)
	layout := &textLayout{
		lineCount: 1,
	}
	if text == "" {
		return layout, nil
	}

	// 每换一行时向下推进的距离 = 字体默认行高 * 行距缩放 * 垂直字形缩放
	lineStep := font.LineHeight() * options.LineSpacingScale * options.GlyphScale.Y()
	if lineStep <= 0 {
		lineStep = float32(font.PixelSize())
	}
	ascender := font.Ascender() * options.GlyphScale.Y()
	descender := font.Descender() * options.GlyphScale.Y()
	maxWidth := float32(0.0)
	lines := strings.Split(text, "\n")
	layout.lineCount = len(lines)
	for lineIndex, line := range lines {
		baselineY := ascender + float32(lineIndex)*lineStep
		lineWidth, err := t.shapeLine(line, font, options, baselineY, &layout.glyphs)
		if err != nil {
			return nil, err
		}
		maxWidth = max(maxWidth, lineWidth)
	}

	height := ascender + descender
	if layout.lineCount > 1 {
		height += float32(layout.lineCount-1) * lineStep
	}
	layout.size = mgl32.Vec2{maxWidth, height}
	return layout, nil
}

// 塑形一行文本
// 对单行文本执行 HarfBuzz shaping 并写入 glyph 绘制位置
func (t *TextRenderer) shapeLine(line string, font abstract.IFont, options *defs.LayoutOptions, baselineY float32, outGlyphs *[]glyphPlacement) (float32, error) {
	if line == "" {
		return 0, nil
	}
	// 取得字体内部的塑形字体
	shapingFont, ok := font.TextShapingFont().(*harfbuzz.Font)
	if !ok || shapingFont == nil {
		return 0, errors.New("font shaping font is nil")
	}

	// 创建 HarfBuzz 塑形缓冲区
	buffer := harfbuzz.NewBuffer()
	// 设置文本方向，例如从左到右或从右到左
	buffer.Props.Direction = t.toHBDirection(t.defaultDirection)
	// 设置文本语言，帮助 HarfBuzz 选择正确的塑形规则
	buffer.Props.Language = language.NewLanguage(t.defaultLanguageTag)
	// 把 UTF-8 字符串转换为 Unicode 码点切片
	runes := []rune(line)
	// 把 Unicode 码点切片添加到 HarfBuzz 塑形缓冲区
	buffer.AddRunes(runes, 0, len(runes))
	// 自动推断尚未指定的文字系统、方向和语言属性
	buffer.GuessSegmentProperties()
	// 使用字体和 OpenType 特性执行塑形，生成字形和位置信息
	buffer.Shape(shapingFont, t.activeFeatures)

	// 初始化画笔的 X 坐标为 0
	penX := float32(0.0)
	// 将画笔的 Y 坐标设置到当前行基线
	penY := baselineY
	// 所有行中字形最左侧的坐标
	lineMinX := float32(0.0)
	// 所有行中字形最右侧的坐标
	lineMaxX := float32(0.0)
	// hasGlyphBounds 用于区分初始值和真实边界
	hasGlyphBounds := false
	// 遍历塑形后生成的所有字形
	for i, info := range buffer.Info {
		position := buffer.Pos[i]
		// 根据字形索引获取字形
		glyph, err := font.TextGlyphByIndex(uint32(info.Glyph))
		if err != nil {
			return 0, err
		}

		// "/ 64" 将 HarfBuzz 的 26.6 定点数转换为浮点像素值
		// Advance 表示绘制后画笔移动的距离
		advanceX := float32(position.XAdvance) / 64.0
		advanceY := float32(position.YAdvance) / 64.0
		// Offset 表示字形相对于画笔位置的偏移量
		offsetX := float32(position.XOffset) / 64.0
		offsetY := float32(position.YOffset) / 64.0
		// 根据布局缩放比例调整画笔移动距离
		scaledAdvanceX := advanceX * options.GlyphScale.X()
		scaledAdvanceY := advanceY * options.GlyphScale.Y()
		// 根据布局缩放比例调整字形偏移量
		scaledOffsetX := offsetX * options.GlyphScale.X()
		scaledOffsetY := offsetY * options.GlyphScale.Y()

		// 只有成功取得字形对象时才计算绘制位置
		if glyph != nil {
			bearing := glyph.GlyphBearing()
			size := glyph.GlyphSize()
			scaledBearingX := bearing.X() * options.GlyphScale.X()
			scaledBearingY := bearing.Y() * options.GlyphScale.Y()
			scaledWidth := size.X() * options.GlyphScale.X()
			scaledHeight := size.Y() * options.GlyphScale.Y()
			// 计算字形左上角的 X/Y 坐标
			dstX := penX + scaledOffsetX + scaledBearingX
			dstY := penY - scaledOffsetY - scaledBearingY
			// 只有字形存在纹理且原始尺寸有效时才加入绘制列表
			if glyph.GlyphTexture() != nil && size.X() > 0 && size.Y() > 0 {
				*outGlyphs = append(*outGlyphs, glyphPlacement{
					glyph: glyph,
					rect:  mgl32.Vec4{dstX, dstY, scaledWidth, scaledHeight},
					uv:    glyph.GlyphUVRect(),
				})
			}
			// 更新这一行所有字形最左侧的坐标
			lineMinX = minLineValue(lineMinX, dstX, hasGlyphBounds)
			// 更新这一行所有字形最右侧的坐标
			lineMaxX = max(lineMaxX, dstX+scaledWidth)
			hasGlyphBounds = true
		}

		penX += scaledAdvanceX
		penY -= scaledAdvanceY
		if i+1 < len(buffer.Info) {
			penX += options.LetterSpacing
		}
	}

	// 找文字最左边和最右边的位置，然后计算它们之间的距离，避免字形伸出画笔范围时宽度计算不完整
	effectiveMin := float32(0.0)
	if hasGlyphBounds {
		effectiveMin = min(float32(0.0), lineMinX)
	}
	effectiveMax := max(float32(0.0), penX)
	if hasGlyphBounds {
		effectiveMax = max(effectiveMax, lineMaxX)
	}
	return max(float32(0.0), effectiveMax-effectiveMin), nil
}

// 转换到 HarfBuzz 文本方向
func (t *TextRenderer) toHBDirection(direction textDirection) harfbuzz.Direction {
	switch direction {
	case textDirectionRightToLeft:
		return harfbuzz.RightToLeft
	case textDirectionTopToBottom:
		return harfbuzz.TopToBottom
	case textDirectionBottomToTop:
		return harfbuzz.BottomToTop
	default:
		return harfbuzz.LeftToRight
	}
}

// 在第一次写入时用当前值初始化行边界
func minLineValue(current, value float32, initialized bool) float32 {
	if !initialized {
		return value
	}
	return min(current, value)
}

// 规范化文本绘制参数
func normalizeTextRenderParams(params *defs.TextRenderParams) *defs.TextRenderParams {
	if params == nil {
		params = &defs.TextRenderParams{}
	} else {
		copied := *params
		params = &copied
	}

	params.Layout = normalizeLayoutOptions(params.Layout)
	params.Color = normalizeTextColorOptions(params.Color)
	return params
}

// 补全文本颜色参数默认值
func normalizeTextColorOptions(options defs.ColorOptions) defs.ColorOptions {
	resolved := options
	if resolved.StartColor == (mgl32.Vec4{}) {
		resolved.StartColor = mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	if resolved.EndColor == (mgl32.Vec4{}) {
		resolved.EndColor = resolved.StartColor
	}
	return resolved
}

// 解析本次绘制最终使用的颜色参数
func resolvedTextColorOptions(params *defs.TextRenderParams) defs.ColorOptions {
	if params == nil {
		return defs.SolidTextColorOptions(mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
	}
	return normalizeTextColorOptions(params.Color)
}

// 规范化布局参数
//
// 当前把行高缩放和 glyph 缩放的零值分量视为 1，方便调用方使用结构体零值
func normalizeLayoutOptions(options *defs.LayoutOptions) *defs.LayoutOptions {
	if options == nil {
		return &defs.LayoutOptions{
			LetterSpacing:    0.0,
			LineSpacingScale: 1.0,
			GlyphScale:       mgl32.Vec2{1.0, 1.0},
		}
	}

	normalized := *options
	if normalized.LineSpacingScale == 0.0 {
		normalized.LineSpacingScale = 1.0
	}
	if normalized.GlyphScale.X() == 0.0 {
		normalized.GlyphScale[0] = 1.0
	}
	if normalized.GlyphScale.Y() == 0.0 {
		normalized.GlyphScale[1] = 1.0
	}
	return &normalized
}
