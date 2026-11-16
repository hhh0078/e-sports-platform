// Package artfonts 私有保存免费商用字体，仅通过有界缓存和并发控制输出实际文案的 WOFF 子集。
package artfonts

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// 原文件没有静态文件路由，不能被客户端整包下载。
//
//go:embed catalog.json sources/*/*.ttf sources/*/OFL.txt
var sources embed.FS

//go:embed subset.py
var subsetScript string

// Font 记录经授权与校验的源字体，不接受请求传入任意路径或网址。
type Font struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// Catalog 前后端共用字体白名单；版本改变时自然失效客户端资源缓存。
type Catalog struct {
	Version string `json:"version"`
	Fonts   []Font `json:"fonts"`
}

var catalog Catalog

// init 在启动时读取随二进制嵌入的固定字体目录。
func init() {
	raw, err := sources.ReadFile("catalog.json")
	if err != nil || json.Unmarshal(raw, &catalog) != nil {
		panic("艺术字体目录无效")
	}
}

// Valid 校验保存的字体编号，空值兼容历史系统字体。
func Valid(id string) bool {
	if id == "" {
		return true
	}
	for _, font := range catalog.Fonts {
		if font.ID == id {
			return true
		}
	}
	return false
}

// canonicalText 对实际文案去重排序；最多144码点覆盖当前和下一张轮播的六条十二字文案。
func canonicalText(text string) (string, bool) {
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > 144 {
		return "", false
	}
	seen := map[rune]bool{}
	for _, char := range text {
		if unicode.IsControl(char) {
			return "", false
		}
		seen[char] = true
	}
	chars := make([]rune, 0, len(seen))
	for char := range seen {
		chars = append(chars, char)
	}
	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })
	return string(chars), len(chars) > 0
}

// cacheEntry 以字形集合缓存不可变资源，内存和条数均有上限。
type cacheEntry struct {
	key  string
	data []byte
	etag string
}

// fontServer 在请求之间复用子集，等待裁剪配额后再次检查已生成缓存。
type fontServer struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List
	bytes   int
	workers chan struct{}
	waiters chan struct{}
}

// Handler 为每个HTTP服务创建独立的16MiB/256项LRU和两个裁剪进程配额。
func Handler() http.Handler {
	server := &fontServer{entries: map[string]*list.Element{}, order: list.New(), workers: make(chan struct{}, 2), waiters: make(chan struct{}, 16)}
	return http.HandlerFunc(server.serve)
}

// cached 读取子集并更新LRU顺序，调用方不修改返回字节。
func (s *fontServer) cached(key string) *cacheEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item := s.entries[key]; item != nil {
		s.order.MoveToFront(item)
		return item.Value.(*cacheEntry)
	}
	return nil
}

// remember 写入有界LRU，淘汰最久未使用内容而非无限增长。
func (s *fontServer) remember(item *cacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous := s.entries[item.key]; previous != nil {
		return
	}
	s.entries[item.key] = s.order.PushFront(item)
	s.bytes += len(item.data)
	for s.order.Len() > 256 || s.bytes > 16<<20 {
		last := s.order.Back()
		old := last.Value.(*cacheEntry)
		delete(s.entries, old.key)
		s.bytes -= len(old.data)
		s.order.Remove(last)
	}
}

// generate 通过无Shell的固定Python程序生成子集，队列、并发和运行时间均受约束。
func (s *fontServer) generate(ctx context.Context, font Font, text, key string) (*cacheEntry, error) {
	select {
	case s.waiters <- struct{}{}:
		defer func() { <-s.waiters }()
	default:
		return nil, fmt.Errorf("字体队列繁忙")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	select {
	case s.workers <- struct{}{}:
		defer func() { <-s.workers }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if item := s.cached(key); item != nil {
		return item, nil
	}
	raw, err := sources.ReadFile(font.File)
	if err != nil {
		return nil, err
	}
	license, err := sources.ReadFile(path.Join(path.Dir(font.File), "OFL.txt"))
	if err != nil {
		return nil, err
	}
	python := os.Getenv("FONT_SUBSET_PYTHON")
	if python == "" {
		python = "python3"
	}
	command := exec.CommandContext(ctx, python, "-c", subsetScript, text, string(license))
	command.Stdin = bytes.NewReader(raw)
	data, err := command.Output()
	if err != nil || len(data) < 44 || len(data) > 512<<10 || !bytes.HasPrefix(data, []byte("wOFF")) {
		return nil, fmt.Errorf("字体裁剪失败")
	}
	sum := sha256.Sum256(data)
	item := &cacheEntry{key: key, data: data, etag: `"` + hex.EncodeToString(sum[:]) + `"`}
	s.remember(item)
	return item, nil
}

// serve 只公开按字形生成的字体，支持跨域匿名读取、ETag和一年浏览器缓存。
func (s *fontServer) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.PathValue("version") != catalog.Version {
		http.Error(w, "字体版本不存在", 404)
		return
	}
	id := strings.TrimSuffix(r.PathValue("font"), ".woff")
	var selected *Font
	for i := range catalog.Fonts {
		if catalog.Fonts[i].ID == id {
			selected = &catalog.Fonts[i]
			break
		}
	}
	if selected == nil || !strings.HasSuffix(r.PathValue("font"), ".woff") {
		http.Error(w, "字体不存在", 404)
		return
	}
	text, valid := canonicalText(r.URL.Query().Get("text"))
	if !valid {
		http.Error(w, "文案需为1至144个有效字符", 400)
		return
	}
	key := id + "/" + text
	item := s.cached(key)
	if item == nil {
		var err error
		item, err = s.generate(r.Context(), *selected, text, key)
		if err != nil {
			w.Header().Set("Retry-After", "2")
			http.Error(w, "字体暂时不可用，请稍后重试", 503)
			return
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", item.etag)
	w.Header().Set("Content-Type", "font/woff")
	if r.Header.Get("If-None-Match") == item.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	http.ServeContent(w, r, "subset.woff", time.Time{}, bytes.NewReader(item.data))
}
