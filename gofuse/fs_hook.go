package gofuse
import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	log "github.com/sirupsen/logrus"
)

type HookFs struct {
	Original   string
	Mountpoint string
	FsName     string
	fs         pathfs.FileSystem
	hook       Hook
}

func NewHookFs(original string, mountpoint string, hook Hook) (*HookFs, error) {
	log.WithFields(log.Fields{
		"original":   original,
		"mountpoint": mountpoint,
	}).Debug("Hooking a fs")

	loopbackfs := pathfs.NewLoopbackFileSystem(original)
	hookfs := &HookFs{
		Original:   original,
		Mountpoint: mountpoint,
		FsName:     "hookfs",
		fs:         loopbackfs,
		hook:       hook,
	}
	return hookfs, nil
}

func (h *HookFs) String() string {
	return fmt.Sprintf("HookFs{Original=%s, Mountpoint=%s, FsName=%s, Underlying fs=%s, hook=%s}",
		h.Original, h.Mountpoint, h.FsName, h.fs.String(), h.hook)
}

func (h *HookFs) SetDebug(debug bool) {
	h.fs.SetDebug(debug)
}

func (h *HookFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	return h.fs.GetAttr(name, context)
}

func (h *HookFs) Chmod(name string, mode uint32, context *fuse.Context) fuse.Status {
	return h.fs.Chmod(name, mode, context)
}

func (h *HookFs) Chown(name string, uid uint32, gid uint32, context *fuse.Context) fuse.Status {
	return h.fs.Chown(name, uid, gid, context)
}

func (h *HookFs) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) fuse.Status {
	return h.fs.Utimens(name, Atime, Mtime, context)
}

func (h *HookFs) Truncate(name string, size uint64, context *fuse.Context) fuse.Status {
	return h.fs.Truncate(name, size, context)
}

func (h *HookFs) Access(name string, mode uint32, context *fuse.Context) fuse.Status {
	return h.fs.Access(name, mode, context)
}

func (h *HookFs) Link(oldName string, newName string, context *fuse.Context) fuse.Status {
	return h.fs.Link(oldName, newName, context)
}

func (h *HookFs) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	return h.fs.Mkdir(name, mode, context)
}

func (h *HookFs) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	return h.fs.Mknod(name, mode, dev, context)
}

func (h *HookFs) Rename(oldName string, newName string, context *fuse.Context) fuse.Status {
	hook, hookEnabled := h.hook.(HookOnRename)
	if hookEnabled {
		preHooked, err := hook.PreRename(oldName, newName)
		if preHooked {
			if err != nil {
				return fuse.ToStatus(err)
			}
		}
	}

	status := h.fs.Rename(oldName, newName, context)

	if hookEnabled {
		postHooked, err := hook.PostRename(oldName, newName)
		if postHooked {
			if err != nil {
				return fuse.ToStatus(err)
			}
		}
	}

	return status
}

func (h *HookFs) Rmdir(name string, context *fuse.Context) fuse.Status {
	return h.fs.Rmdir(name, context)
}

func (h *HookFs) Unlink(name string, context *fuse.Context) fuse.Status {
	return h.fs.Unlink(name, context)
}

func (h *HookFs) GetXAttr(name string, attribute string, context *fuse.Context) ([]byte, fuse.Status) {
	return h.fs.GetXAttr(name, attribute, context)
}

func (h *HookFs) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	return h.fs.ListXAttr(name, context)
}

func (h *HookFs) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	return h.fs.RemoveXAttr(name, attr, context)
}

func (h *HookFs) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return h.fs.SetXAttr(name, attr, data, flags, context)
}

func (h *HookFs) OnMount(nodeFs *pathfs.PathNodeFs) {
	h.fs.OnMount(nodeFs)
}

func (h *HookFs) OnUnmount() {
	h.fs.OnUnmount()
}

func (h *HookFs) Open(name string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	return h.fs.Open(name, flags, context)
}

func (h *HookFs) Create(name string, flags uint32, mode uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	return h.fs.Create(name, flags, mode, context)
}

func (h *HookFs) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	return h.fs.OpenDir(name, context)
}

func (h *HookFs) Symlink(value string, linkName string, context *fuse.Context) fuse.Status {
	return h.fs.Symlink(value, linkName, context)
}

func (h *HookFs) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	return h.fs.Readlink(name, context)
}

func (h *HookFs) StatFs(name string) *fuse.StatfsOut {
	return h.fs.StatFs(name)
}

// Serve initiates the FUSE loop. Normally, callers should run Serve()
// and wait for it to exit, but tests will want to run this in a
// goroutine.
func (h *HookFs) ServeAsync() (*fuse.Server, error) {
	server, err := newHookServer(h)
	if err != nil {
		return nil, err
	}

	go func() {
		server.Serve()
	}()

	return server, nil
}

func newHookServer(hookfs *HookFs) (*fuse.Server, error) {
	opts := &nodefs.Options{
		NegativeTimeout: time.Second,
		AttrTimeout:     time.Second,
		EntryTimeout:    time.Second,
	}
	pathFsOpts := &pathfs.PathNodeFsOptions{ClientInodes: true}
	pathFs := pathfs.NewPathNodeFs(hookfs, pathFsOpts)
	conn := nodefs.NewFileSystemConnector(pathFs.Root(), opts)
	originalAbs, _ := filepath.Abs(hookfs.Original)
	mOpts := &fuse.MountOptions{
		AllowOther: true,
		Name:       hookfs.FsName,
		FsName:     originalAbs,
	}
	server, err := fuse.NewServer(conn.RawFS(), hookfs.Mountpoint, mOpts)
	if err != nil {
		return nil, err
	}

	return server, nil
}

