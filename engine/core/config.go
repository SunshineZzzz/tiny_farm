package core

import (
	"encoding/json"
	"os"
)

// 对应配置 JSON 的根结构
type configJson struct {
	Window      windowConfig      `json:"window"`
	Graphics    graphicsConfig    `json:"graphics"`
	Performance performanceConfig `json:"performance"`
}

// 对应 window 配置
type windowConfig struct {
	Title        string  `json:"title"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	WindowScale  float32 `json:"window_scale"`
	LogicalScale float32 `json:"logical_scale"`
	Resizable    bool    `json:"resizable"`
}

// 对应 graphics 配置
type graphicsConfig struct {
	Vsync   bool `json:"vsync"`
	DebugUI bool `json:"debug_ui"`
}

// 对应 performance 配置
type performanceConfig struct {
	TargetFPS int `json:"target_fps"`
}

// 管理应用程序配置
type Config struct {
	configJson
}

// 创建配置并从文件加载覆盖项
func NewConfig(filePath string) (*Config, error) {
	config := &Config{}
	config.Init()
	if err := config.LoadFromFile(filePath); err != nil {
		return nil, err
	}
	return config, nil
}

// 设置配置默认值
func (c *Config) Init() {
	c.Window.Title = "TinyFarm"
	c.Window.Width = 1280
	c.Window.Height = 720
	c.Window.WindowScale = 1.4
	c.Window.LogicalScale = 0.5
	c.Window.Resizable = true
	c.Graphics.Vsync = true
	c.Graphics.DebugUI = true
	c.Performance.TargetFPS = 60
}

// 从文件中加载配置
func (c *Config) LoadFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if err = c.SaveToFile(filePath); err != nil {
			return err
		}
		return nil
	}

	var config configJson
	if err = json.Unmarshal(data, &config); err != nil {
		return err
	}

	// 应用解析后的配置
	c.Window.Title = config.Window.Title
	c.Window.Width = config.Window.Width
	c.Window.Height = config.Window.Height
	c.Window.WindowScale = config.Window.WindowScale
	c.Window.LogicalScale = config.Window.LogicalScale
	c.Window.Resizable = config.Window.Resizable
	c.Graphics.Vsync = config.Graphics.Vsync
	c.Graphics.DebugUI = config.Graphics.DebugUI
	c.Performance.TargetFPS = config.Performance.TargetFPS
	return nil
}

// 保存配置到文件
func (c *Config) SaveToFile(filePath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	return nil
}
