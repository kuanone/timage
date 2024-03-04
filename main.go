package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

type FileType interface {
	// IsType 判断文件是否属于该类型
	IsType(file string) bool
	Type() string
}

type JPGFileType struct{}

func (j *JPGFileType) IsType(file string) bool {
	return filepath.Ext(file) == ".jpg" || filepath.Ext(file) == ".jpeg"
}

func (j *JPGFileType) Type() string {
	return "JPG"
}

type PNGFileType struct{}

func (p *PNGFileType) IsType(file string) bool {
	return filepath.Ext(file) == ".png"
}

func (p *PNGFileType) Type() string {
	return "PNG"
}

type FilterOptions struct {
	// 是否过滤隐藏文件
	FilterHidden bool
	// 是否获取文件大小
	GetFileSize bool
	// 是否获取文件MD5
	GetFileMD5 bool
	// 是否获取文件元信息
	GetFileMeta bool
}

type Filter[T FileType] struct {
	// 文件类型
	fileType T
	// 过滤选项
	options FilterOptions
}

func NewFilter[T FileType](fileType T, options FilterOptions) *Filter[T] {
	return &Filter[T]{
		fileType: fileType,
		options:  options,
	}
}

// FilterFiles 过滤文件
func (f *Filter[T]) FilterFiles(files []string) []string {
	var filteredFiles []string
	for _, file := range files {
		if f.options.FilterHidden && isHiddenFile(file) {
			continue
		}
		if strings.Contains(file, "compressed") {
			continue
		}
		if f.fileType.IsType(file) {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles
}

func isHiddenFile(file string) bool {
	return filepath.Base(file)[0] == '.'
}

type FileInfo struct {
	// 文件名
	Name string
	// 文件大小
	Size int64
	// 文件MD5
	MD5 string
	// 文件元信息
	Meta map[string]string
}

func GetFileInfo(file string, options FilterOptions) (FileInfo, error) {
	info, err := os.Stat(file)
	if err != nil {
		return FileInfo{}, err
	}

	fileInfo := FileInfo{
		Name: info.Name(),
		Size: info.Size(),
	}

	if options.GetFileMD5 {
		fileMd5, err := GetFileMD5(file)
		if err != nil {
			return FileInfo{}, err
		}
		fileInfo.MD5 = fileMd5
	}

	if options.GetFileMeta {
		meta, err := GetFileMeta(file)
		if err != nil {
			return FileInfo{}, err
		}
		fileInfo.Meta = meta
	}

	return fileInfo, nil
}

func GetFileMD5(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}(f)

	md5Hash := md5.New()
	_, err = io.Copy(md5Hash, f)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", md5.Sum(nil)), nil
}

func GetFileMeta(file string) (map[string]string, error) {
	// todo: 获取文件元信息 https://github.com/rwcarlsen/goexif
	return nil, nil
}

type FileIterator[T any] func(file string) T

func IterateFiles[T any](dir string, iterator FileIterator[T]) []T {
	var results []T
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			results = append(results, iterator(path))
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return results
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	println(dir)

	// 创建 JPG 和 PNG 文件类型
	jpgFileType := &JPGFileType{}
	pngFileType := &PNGFileType{}

	// 创建过滤选项
	options := FilterOptions{
		FilterHidden: true,
		GetFileSize:  true,
		GetFileMD5:   true,
	}

	// 创建过滤器
	jpgFilter := NewFilter(jpgFileType, options)
	pngFilter := NewFilter(pngFileType, options)

	// 使用泛型遍历文件
	files := IterateFiles(dir, func(file string) string {
		return file
	})

	// 过滤 JPG 文件
	jpgFiles := jpgFilter.FilterFiles(files)

	// 过滤 PNG 文件
	pngFiles := pngFilter.FilterFiles(files)

	// 打印结果
	fmt.Println("JPG 文件：")
	printPrettyInfo(jpgFiles, options)

	fmt.Println("PNG 文件：")
	printPrettyInfo(pngFiles, options)

	// 压缩图片
	compressionOptions := CompressionOptions{
		CompressImage: true,
	}
	if compressionOptions.CompressImage {
		compressedJpgFiles, compressedPngFiles, err2 := compressImages(jpgFiles, pngFiles, compressionOptions)
		if err2 != nil {
			fmt.Println("压缩失败：", err2)
			return
		}
		printPrettyInfo(compressedJpgFiles, options)
		printPrettyInfo(compressedPngFiles, options)
	}
}

func compressImages(jpgFiles, pngFiles []string, options CompressionOptions) (compressedJpgFiles, compressedPngFiles []string, err error) {
	// 创建图片压缩器
	compressor := &DefaultImageCompressor{
		Options: options.ImageOptions,
	}

	fOption := FilterOptions{
		GetFileSize: true,
		GetFileMD5:  true,
	}

	// 压缩 JPG 文件
	for _, file := range jpgFiles {
		fileInfo, err1 := GetFileInfo(file, fOption)
		if err1 != nil {
			return compressedJpgFiles, compressedPngFiles, err1
		}
		image, err1 := compressor.compressImage(file, &fileInfo)
		if err1 != nil {
			return compressedJpgFiles, compressedPngFiles, err1
		}
		compressedJpgFiles = append(compressedJpgFiles, image)
	}

	// 压缩 PNG 文件
	for _, file := range pngFiles {
		fileInfo, err1 := GetFileInfo(file, fOption)
		if err1 != nil {
			return compressedJpgFiles, compressedPngFiles, err1
		}
		image, err1 := compressor.compressImage(file, &fileInfo)
		if err1 != nil {
			return compressedJpgFiles, compressedPngFiles, err1
		}
		compressedPngFiles = append(compressedPngFiles, image)
	}
	return compressedJpgFiles, compressedPngFiles, nil
}

func printPrettyInfo(files []string, options FilterOptions) {
	// 根据options获取文件信息，打印到控制台
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	// 设置表头
	var header table.Row
	header = append(header, "#")
	header = append(header, "文件名")
	if options.GetFileSize {
		header = append(header, "文件大小")
	}
	if options.GetFileMD5 {
		header = append(header, "文件MD5")
	}
	if options.GetFileMeta {
		header = append(header, "文件元信息")
	}

	t.AppendHeader(header)
	for i, file := range files {
		fileInfo, err := GetFileInfo(file, options)
		if err != nil {
			panic(err)
		}
		var row table.Row
		row = append(row, i+1)
		row = append(row, fileInfo.Name)

		if options.GetFileSize {
			row = append(row, humanSize(fileInfo.Size))
		}
		if options.GetFileMD5 {
			row = append(row, fileInfo.MD5)
		}
		if options.GetFileMeta {
			row = append(row, fileInfo.Meta)
		}
		t.AppendRow(row)
	}
	t.Render()
}

type CompressionOptions struct {
	// 是否压缩图片
	CompressImage bool
	// 图片压缩选项
	ImageOptions ImageOptions
}

type CloudOptions struct {
	// 云存储类型
	Type string
	// 云存储地址
	Address string
	// 云存储账号
	Account string
	// 云存储密码
	Password string
	// 云存储Token
	Token string
	// 云存储Bucket
	Bucket string
	// 云存储Region
	Region string
	// 云存储路径
	Path string
}

type ImageOptions struct {
	// 压缩质量
	Quality int
	// 是否保留原图
	KeepOriginal bool
	// 是否自动上传到云存储
	AutoUpload bool
	// 云存储选项
	CloudOptions CloudOptions
	// 图片压缩器
	Compressor FileCompressor
}

// FileCompressor 文件压缩器
type FileCompressor interface {
	// compressImage 压缩图片，返回压缩后的图片路径；如果压缩失败，返回错误；
	compressImage(file string, fileInfo *FileInfo) (string, error)
}

type DefaultImageCompressor struct {
	// 图片压缩选项
	Options ImageOptions
}

func (i *DefaultImageCompressor) compressImage(file string, fileInfo *FileInfo) (string, error) {
	if filepath.Ext(file) == ".jpg" || filepath.Ext(file) == ".jpeg" {
		return tinyPngCompress(file, "compressed/"+filepath.Base(file))
	} else if filepath.Ext(file) == ".png" {
		return tinyPngCompress(file, "compressed/"+filepath.Base(file))
	}
	return "", fmt.Errorf("不支持的图片类型：%s", file)
}

// humanSize 格式化文件大小
func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

const (
	// 替换为你的 API 密钥
	apiKey = "----"

	// 上传图片 URL
	uploadUrl = "https://api.tinify.com/shrink"
)

func tinyPngCompress(input, output string) (string, error) {
	// 读取本地图片
	file, err := os.Open(input)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", uploadUrl, nil)
	if err != nil {
		return "", err
	}

	// 设置请求头
	req.SetBasicAuth("api", apiKey)

	req.Body = io.NopCloser(file)

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 读取压缩后的图片
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	response := TinifyShrinkResponse{}
	err = json.Unmarshal(data, &response)
	if err != nil {
		return "", err
	}

	if response.Error != "" {
		return "", fmt.Errorf("压缩失败：%s", response.Message)
	}

	if response.Output.URL == "" {
		return "", fmt.Errorf("压缩失败：%s", response.Message)
	}

	fResponse, err := http.DefaultClient.Get(response.Output.URL)
	if err != nil {
		return "", err
	}
	defer fResponse.Body.Close()
	data, err = io.ReadAll(fResponse.Body)
	if err != nil {
		return "", err
	}

	// 保存压缩后的图片
	err = SaveFile(output, data)
	return output, err
}

type TinifyShrinkResponse struct {
	Input   *Input  `json:"input,omitempty"`
	Output  *Output `json:"output,omitempty"`
	Message string  `json:"message,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type Input struct {
	Size int    `json:"size"`
	Type string `json:"type"`
}

type Output struct {
	Size   int     `json:"size"`
	Type   string  `json:"type"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Ratio  float64 `json:"ratio"`
	URL    string  `json:"url"`
}

func SaveFile(fileName string, data []byte) error {
	dir := filepath.Dir(fileName)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	if err != nil {
		slog.Error("SaveFile file create failed.", "err", err.Error())
	} else {
		// offset
		// os.Truncate(filename, 0) //clear
		n, _ := f.Seek(0, io.SeekEnd)
		_, err = f.WriteAt(data, n)
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				slog.Error("SaveFile close failed.", "err", err.Error())
			}
		}(f)
	}
	return err
}
