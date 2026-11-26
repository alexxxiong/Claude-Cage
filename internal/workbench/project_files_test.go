package workbench

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectFileTransferRoundTrip(t *testing.T) {
	s := testServer(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.rememberSharedProject(root); err != nil {
		t.Fatal(err)
	}
	const payload = "Cage 项目\x00\xff\r\n"
	send := func(workspace, directory, action, name, method, body string, status int) string {
		t.Helper()
		q := url.Values{"workspace": {workspace}, "path": {directory}, "action": {action}, "name": {name}}
		w := request(s, method, "/api/projects/files?"+q.Encode(), s.origin, body, true)
		if w.Code != status {
			t.Fatalf("%s %s %s: %d %s", directory, action, name, w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	send(root, ".", "mkdir", "新文件夹", "POST", "", 200)
	send(root, "新文件夹", "upload", "例子.txt", "POST", payload, 200)
	send(root, "新文件夹", "upload", "例子.txt", "POST", "other", 409)
	if got := send(root, "新文件夹", "download", "例子.txt", "GET", "", 200); got != payload {
		t.Fatal("download differs")
	}
	if got := send(root, "新文件夹", "", "", "GET", "", 200); !strings.Contains(got, "例子.txt") || strings.Contains(got, ".cage-upload-") {
		t.Fatal(got)
	}
	send(t.TempDir(), ".", "", "", "GET", "", 403)
	send(root, ".", "mkdir", ".git", "POST", "", 400)
	s.account.lock()
	send(root, ".", "", "", "GET", "", 423)
}
