package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
)

// go run . -in ../../assets/farm-rpg/UI/button.png -out slice_button_normalandhover.png -x 0 -y 16 -width 48 -height 16
// go run . -in ../../assets/farm-rpg/UI/button.png -out slice_button_pressed.png -x 0 -y 32 -width 48 -height 16

func main() {
	input := flag.String("in", "", "输入 PNG 图片路径")
	output := flag.String("out", "", "输出 PNG 图片路径")
	x := flag.Int("x", 0, "裁剪区域左上角 X")
	y := flag.Int("y", 0, "裁剪区域左上角 Y")
	width := flag.Int("width", 0, "裁剪区域宽度")
	height := flag.Int("height", 0, "裁剪区域高度")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "用法: go run ./tools/imgSlice -in <输入.png> -out <输出.png> -x <x> -y <y> -width <宽度> -height <高度>\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "示例: go run ./tools/imgSlice -in assets/farm-rpg/UI/button.png -out slice_button_normal.png -x 0 -y 16 -width 48 -height 16\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *input == "" || *output == "" || *width <= 0 || *height <= 0 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*input, *output, image.Rect(*x, *y, *x+*width, *y+*height)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output string, rect image.Rectangle) error {
	file, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("打开输入图片失败: %w", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("解析 PNG 失败: %w", err)
	}

	bounds := img.Bounds()
	if !rect.In(bounds) {
		return fmt.Errorf("裁剪区域 %v 超出图片范围 %v", rect, bounds)
	}

	source, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		return fmt.Errorf("输入图片类型不支持 SubImage")
	}

	outFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("创建输出图片失败: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, source.SubImage(rect)); err != nil {
		return fmt.Errorf("写入 PNG 失败: %w", err)
	}
	return nil
}
