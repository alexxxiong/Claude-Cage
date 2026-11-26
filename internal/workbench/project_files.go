package workbench

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sylvinus/agent-vm/internal/mounts"
)

// Project transfers operate on the registered host folder, just like an editor
// saving into the shared mount. os.Root confines subsequent path operations.
func (s *Server) projectFiles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := q.Get("workspace")
	folder, err := s.projectWorkspace(scope)
	if err != nil || scope == "" || isClientMountPath(scope) || filepath.Clean(folder) != scope {
		http.Error(w, "请先绑定此项目目录", 403)
		return
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		http.Error(w, "项目目录无法访问", 409)
		return
	}
	defer root.Close()
	relative := q.Get("path")
	if relative == "" {
		relative = "."
	}
	if !fs.ValidPath(relative) || strings.ContainsAny(relative, "\\:\x00") {
		http.Error(w, "目录名称无效", 400)
		return
	}
	parent, err := openProjectSubdirectory(root, relative)
	if err != nil {
		http.Error(w, "目录无法访问", 404)
		return
	}
	defer parent.Close()
	action := q.Get("action")
	if r.Method == "GET" && action == "" {
		dir, err := parent.Open(".")
		if err != nil {
			http.Error(w, "无法读取目录", 403)
			return
		}
		defer dir.Close()
		entries, err := dir.ReadDir(10001)
		if err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "无法读取目录", 403)
			return
		}
		if len(entries) > 10000 {
			http.Error(w, "目录超过一万项，请选择更小的目录", 413)
			return
		}
		type entry struct {
			Name string `json:"name"`
			Dir  bool   `json:"dir"`
			Size int64  `json:"size"`
		}
		files := []entry{}
		for _, item := range entries {
			if !projectLeaf(item.Name()) {
				continue
			}
			info, e := item.Info()
			if e == nil && (info.IsDir() || info.Mode().IsRegular()) {
				files = append(files, entry{item.Name(), info.IsDir(), info.Size()})
			}
		}
		sort.Slice(files, func(i, j int) bool {
			if files[i].Dir != files[j].Dir {
				return files[i].Dir
			}
			return files[i].Name < files[j].Name
		})
		jsonReply(w, map[string]any{"files": files})
		return
	}
	name := q.Get("name")
	if !projectLeaf(name) {
		http.Error(w, "请输入单个文件或文件夹名称", 400)
		return
	}
	switch {
	case r.Method == "POST" && action == "mkdir":
		err = parent.Mkdir(name, 0700)
	case r.Method == "POST" && action == "upload":
		const limit = 2 << 30
		if r.ContentLength > limit {
			http.Error(w, "单个文件最大 2 GiB", 413)
			return
		}
		tmp := ".cage-upload-" + randomToken()
		file, e := parent.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			http.Error(w, "无法创建上传文件", 409)
			return
		}
		defer parent.Remove(tmp)
		_, err = io.Copy(file, http.MaxBytesReader(w, r.Body, limit))
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = parent.Link(tmp, name)
		}
	case r.Method == "GET" && action == "download":
		info, e := parent.Lstat(name)
		if e != nil || !info.Mode().IsRegular() {
			http.Error(w, "只能下载普通文件", 404)
			return
		}
		file, e := parent.Open(name)
		if e != nil {
			http.Error(w, "文件无法打开", 403)
			return
		}
		defer file.Close()
		current, e := file.Stat()
		if e != nil || !os.SameFile(info, current) {
			http.Error(w, "文件已变化，请刷新", 409)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, name, current.ModTime(), file)
		return
	default:
		http.Error(w, "不支持的文件操作", 400)
		return
	}
	if errors.Is(err, fs.ErrExist) {
		http.Error(w, "同名文件或目录已存在", 409)
		return
	}
	if err != nil {
		http.Error(w, "操作未完成，请检查空间、名称和写入权限", 409)
		return
	}
	jsonReply(w, map[string]bool{"ok": true})
}

func openProjectSubdirectory(root *os.Root, relative string) (*os.Root, error) {
	current, err := root.OpenRoot(".")
	if err != nil || relative == "." {
		return current, err
	}
	for _, part := range strings.Split(relative, "/") {
		if !projectLeaf(part) {
			current.Close()
			return nil, fs.ErrPermission
		}
		info, err := current.Lstat(part)
		if err != nil || !info.IsDir() {
			current.Close()
			return nil, fs.ErrPermission
		}
		next, err := current.OpenRoot(part)
		current.Close()
		if err != nil {
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			next.Close()
			return nil, fs.ErrPermission
		}
		current = next
	}
	return current, nil
}

func projectLeaf(name string) bool {
	return !mounts.UnderName(name, nil) && !strings.HasPrefix(name, ".cage-upload-") && fs.ValidPath(name) && name != "." && path.Base(name) == name && filepath.IsLocal(name) && !strings.ContainsAny(name, "\\:\x00\r\n")
}

func (s *Server) newProjectFolder(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if !projectLeaf(input.Name) || strings.HasPrefix(input.Name, ".") {
		http.Error(w, "请输入有效的项目名称", 400)
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "无法定位项目目录", 500)
		return
	}
	parent := filepath.Join(home, "Project")
	if err = os.Mkdir(parent, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		http.Error(w, "无法创建项目根目录", 403)
		return
	}
	real, err := validateWorkspace(parent, s.profile)
	if err != nil || real != parent {
		http.Error(w, "项目根目录不可用", 403)
		return
	}
	root, err := os.OpenRoot(real)
	if err != nil {
		http.Error(w, "无法打开项目根目录", 403)
		return
	}
	defer root.Close()
	if err = root.Mkdir(input.Name, 0700); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			http.Error(w, "无法创建目录", 409)
			return
		}
		info, e := root.Lstat(input.Name)
		if e != nil || !info.IsDir() {
			http.Error(w, "同名文件已存在", 409)
			return
		}
	}
	jsonReply(w, map[string]string{"path": filepath.Join(parent, input.Name)})
}
