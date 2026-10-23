package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

var user = syscall.NewLazyDLL("user32.dll")
var create = user.NewProc("CreateWindowExW")
var send = user.NewProc("SendMessageW")
var setText = user.NewProc("SetWindowTextW")
var list, status, window uintptr
var mu sync.Mutex
var readers sync.WaitGroup
var views = map[string]*View{}
var pending = map[string]time.Time{}
var directoryError string
var displayRuns []Run
var renderedRows [][]string
var directory string
var ctx context.Context
var cancel context.CancelFunc

func u(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(s, "\x00", "�"))
	return p
}

type wndclass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type point struct{ X, Y int32 }
type message struct {
	Window  uintptr
	ID      uint32
	W, L    uintptr
	Time    uint32
	Point   point
	Private uint32
}
type column struct {
	Mask                                                               uint32
	Format, Width                                                      int32
	Text                                                               *uint16
	TextMax, Subitem, Image, Order, MinWidth, DefaultWidth, IdealWidth int32
}
type item struct {
	Mask                 uint32
	Index, Subitem       int32
	State, StateMask     uint32
	Text                 *uint16
	TextMax, Image       int32
	Param                uintptr
	Indent, GroupID      int32
	Columns              uint32
	ColumnPtr, FormatPtr uintptr
	Group                int32
}
type notify struct {
	Window, ID uintptr
	Code       uint32
}

func newWindow(class, title string, style uint32, x, y, w, h int, parent, id uintptr) uintptr {
	r, _, _ := create.Call(0, uintptr(unsafe.Pointer(u(class))), uintptr(unsafe.Pointer(u(title))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, 0, 0)
	return r
}
func short(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}
func refresh() {
	mu.Lock()
	now, tick := time.Now().UTC(), ticks()
	sources := make([]string, 0, len(views))
	for k := range views {
		sources = append(sources, k)
	}
	sort.Strings(sources)
	var rows [][]string
	var nextRuns []Run
	labels := []string{}
	for _, key := range sources {
		v := views[key]
		label := short(key)
		if v.Snapshot == nil {
			labels = append(labels, label+": "+v.Error)
			continue
		}
		stale := v.Stale(now, tick)
		label += " " + v.Snapshot.Generated.Local().Format("15:04:05")
		if stale {
			label += " [expired]"
		}
		if v.Error != "" {
			label += " [" + v.Error + "]"
		}
		labels = append(labels, label)
		for _, r := range v.Snapshot.Runs {
			statusText := r.Status
			if stale || v.Error != "" || directoryError != "" {
				statusText = "last: " + statusText
			}
			pr := "—"
			if r.PR != nil {
				pr = fmt.Sprintf("#%d", r.PR.Number)
			}
			values := []string{r.Title + "  [" + short(r.ID) + " / " + short(key) + "]", fmt.Sprintf("%s/%d · %s", r.Workflow.ID, r.Workflow.Version, r.Step), statusText, pr}
			rows = append(rows, values)
			nextRuns = append(nextRuns, r)
		}
	}
	if directoryError != "" {
		labels = append(labels, directoryError)
	}
	if len(labels) == 0 {
		labels = append(labels, "No snapshots")
	}
	mu.Unlock() // never hold the reader gate across reentrant window messages
	setText.Call(status, uintptr(unsafe.Pointer(u(strings.Join(labels, "  |  ")))))
	// Leave rows/selection alone when only the freshness clock ticks.
	displayRuns = nextRuns
	if reflect.DeepEqual(rows, renderedRows) {
		return
	}
	renderedRows = rows
	send.Call(list, 0x000B, 0, 0)
	send.Call(list, 0x1009, 0, 0)
	for row, values := range rows {
		for sub, text := range values {
			it := item{Mask: 1, Index: int32(row), Subitem: int32(sub), Text: u(text)}
			if sub == 0 {
				send.Call(list, 0x104D, 0, uintptr(unsafe.Pointer(&it)))
			} else {
				send.Call(list, 0x1074, uintptr(row), uintptr(unsafe.Pointer(&it)))
			}
		}
	}
	send.Call(list, 0x000B, 1, 0)
	user.NewProc("InvalidateRect").Call(list, 0, 1)
}
func poll() {
	entries, err := os.ReadDir(directory)
	mu.Lock()
	if err != nil {
		if os.IsNotExist(err) && len(views) == 0 {
			directoryError = "No snapshots"
		} else {
			directoryError = "Cannot read snapshot directory"
		}
		mu.Unlock()
		return
	}
	directoryError = ""
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".json")
		if !sourcePattern.MatchString(key) {
			continue
		}
		seen[key] = true
		if views[key] == nil {
			views[key] = &View{}
		}
		if started, ok := pending[key]; ok {
			if time.Since(started) > 2*time.Second {
				views[key].Error = "read timed out"
			}
			continue
		}
		pending[key] = time.Now()
		name := e.Name()
		readers.Add(1)
		go func() {
			defer readers.Done()
			bounded, done := context.WithTimeout(ctx, 2*time.Second)
			defer done()
			body, err := readSnapshot(bounded, filepath.Join(directory, name))
			var s Snapshot
			if err == nil {
				s, err = Parse(body, name)
			}
			mu.Lock()
			defer mu.Unlock()
			delete(pending, key)
			if bounded.Err() != nil {
				err = bounded.Err()
			}
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				views[key].Error = "cannot read valid snapshot"
			} else {
				_ = views[key].Accept(s, time.Now().UTC(), ticks())
			}
		}()
	}
	for key, v := range views {
		if !seen[key] {
			v.Error = "snapshot missing"
		}
	}
	mu.Unlock()
}
func proc(hwnd uintptr, msg uint32, w uintptr, l unsafe.Pointer) uintptr {
	switch msg {
	case 0x0005: // WM_SIZE
		width, height := int(uintptr(l)&0xffff), int((uintptr(l)>>16)&0xffff)
		user.NewProc("MoveWindow").Call(status, 10, 8, uintptr(width-20), 38, 1)
		user.NewProc("MoveWindow").Call(list, 10, 48, uintptr(width-20), uintptr(height-58), 1)
		return 0
	case 0x0113:
		refresh()
		return 0
	case 0x000F: // WM_PAINT: recompute before drawing a resumed cached window
		if list != 0 && status != 0 {
			refresh()
		}
	case 0x004E:
		n := (*notify)(l)
		if n != nil && n.Window == list && n.Code == ^uint32(2) {
			row, _, _ := send.Call(list, 0x100C, ^uintptr(0), 2)
			if int(row) >= 0 && int(row) < len(displayRuns) {
				r := displayRuns[int(row)]
				if r.PR != nil && SafeURL(r.PR.URL) {
					syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(hwnd, uintptr(unsafe.Pointer(u("open"))), uintptr(unsafe.Pointer(u(r.PR.URL))), 0, 0, 1)
				}
			}
		}
		return 0
	case 0x0002:
		cancel()
		user.NewProc("PostQuitMessage").Call(0)
		return 0
	}
	r, _, _ := user.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), w, uintptr(l))
	return r
}
func main() {
	defaultDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "SteerRunnerView", "snapshots")
	flag.StringVar(&directory, "snapshot-dir", defaultDir, "local snapshot directory")
	flag.Parse()
	if !filepath.IsAbs(directory) || strings.HasPrefix(directory, `\\`) {
		return
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	runtime.LockOSThread()
	common := struct{ Size, Classes uint32 }{8, 1}
	syscall.NewLazyDLL("comctl32.dll").NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&common)))
	wc := wndclass{Proc: syscall.NewCallback(proc), Background: 6, Name: u("SteerRunnerReadOnlyViewer")}
	wc.Size = uint32(unsafe.Sizeof(wc))
	user.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	window = newWindow("SteerRunnerReadOnlyViewer", "SteerRunner — Run viewer", 0x00CF0000|0x10000000, 100, 100, 1100, 620, 0, 0)
	if window == 0 {
		return
	}
	status = newWindow("STATIC", "Reading snapshots…", 0x50000000, 10, 8, 1060, 38, window, 1)
	list = newWindow("SysListView32", "", 0x50000000|0x00800000|1|4, 10, 48, 1060, 510, window, 2)
	send.Call(list, 0x1036, 0, 0x20|1)
	for i, text := range []string{"Run", "Workflow", "Status", "PR"} {
		c := column{Mask: 6, Width: []int32{440, 280, 190, 100}[i], Text: u(text)}
		send.Call(list, 0x1061, uintptr(i), uintptr(unsafe.Pointer(&c)))
	}
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		timer := time.NewTicker(2 * time.Second)
		defer timer.Stop()
		for {
			poll()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
	user.NewProc("SetTimer").Call(window, 1, 250, 0)
	var m message
	for {
		r, _, _ := user.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		user.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		user.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}
	cancel()
	<-pollDone // no new readers may be added after this point
	readers.Wait()
}
