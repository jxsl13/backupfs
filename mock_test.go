// Code generated by MockGen. DO NOT EDIT.
// Source: filesystem.go
//
// Generated by this command:
//
//	mockgen -package=backupfs -source=filesystem.go -destination=./mock_test.go FS
//

// Package backupfs is a generated GoMock package.
package backupfs

import (
	fs "io/fs"
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"
)

// MockFS is a mock of FS interface.
type MockFS struct {
	ctrl     *gomock.Controller
	recorder *MockFSMockRecorder
}

// MockFSMockRecorder is the mock recorder for MockFS.
type MockFSMockRecorder struct {
	mock *MockFS
}

// NewMockFS creates a new mock instance.
func NewMockFS(ctrl *gomock.Controller) *MockFS {
	mock := &MockFS{ctrl: ctrl}
	mock.recorder = &MockFSMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFS) EXPECT() *MockFSMockRecorder {
	return m.recorder
}

// Chmod mocks base method.
func (m *MockFS) Chmod(name string, mode fs.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chmod", name, mode)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chmod indicates an expected call of Chmod.
func (mr *MockFSMockRecorder) Chmod(name, mode any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chmod", reflect.TypeOf((*MockFS)(nil).Chmod), name, mode)
}

// Chown mocks base method.
func (m *MockFS) Chown(name string, uid, gid int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chown", name, uid, gid)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chown indicates an expected call of Chown.
func (mr *MockFSMockRecorder) Chown(name, uid, gid any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chown", reflect.TypeOf((*MockFS)(nil).Chown), name, uid, gid)
}

// Chtimes mocks base method.
func (m *MockFS) Chtimes(name string, atime, mtime time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chtimes", name, atime, mtime)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chtimes indicates an expected call of Chtimes.
func (mr *MockFSMockRecorder) Chtimes(name, atime, mtime any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chtimes", reflect.TypeOf((*MockFS)(nil).Chtimes), name, atime, mtime)
}

// Create mocks base method.
func (m *MockFS) Create(name string) (File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", name)
	ret0, _ := ret[0].(File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockFSMockRecorder) Create(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockFS)(nil).Create), name)
}

// Lchown mocks base method.
func (m *MockFS) Lchown(name string, uid, gid int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lchown", name, uid, gid)
	ret0, _ := ret[0].(error)
	return ret0
}

// Lchown indicates an expected call of Lchown.
func (mr *MockFSMockRecorder) Lchown(name, uid, gid any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lchown", reflect.TypeOf((*MockFS)(nil).Lchown), name, uid, gid)
}

// Lstat mocks base method.
func (m *MockFS) Lstat(name string) (fs.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lstat", name)
	ret0, _ := ret[0].(fs.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Lstat indicates an expected call of Lstat.
func (mr *MockFSMockRecorder) Lstat(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lstat", reflect.TypeOf((*MockFS)(nil).Lstat), name)
}

// Mkdir mocks base method.
func (m *MockFS) Mkdir(name string, perm fs.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mkdir", name, perm)
	ret0, _ := ret[0].(error)
	return ret0
}

// Mkdir indicates an expected call of Mkdir.
func (mr *MockFSMockRecorder) Mkdir(name, perm any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mkdir", reflect.TypeOf((*MockFS)(nil).Mkdir), name, perm)
}

// MkdirAll mocks base method.
func (m *MockFS) MkdirAll(path string, perm fs.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MkdirAll", path, perm)
	ret0, _ := ret[0].(error)
	return ret0
}

// MkdirAll indicates an expected call of MkdirAll.
func (mr *MockFSMockRecorder) MkdirAll(path, perm any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MkdirAll", reflect.TypeOf((*MockFS)(nil).MkdirAll), path, perm)
}

// Name mocks base method.
func (m *MockFS) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockFSMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockFS)(nil).Name))
}

// Open mocks base method.
func (m *MockFS) Open(name string) (File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Open", name)
	ret0, _ := ret[0].(File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Open indicates an expected call of Open.
func (mr *MockFSMockRecorder) Open(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Open", reflect.TypeOf((*MockFS)(nil).Open), name)
}

// OpenFile mocks base method.
func (m *MockFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OpenFile", name, flag, perm)
	ret0, _ := ret[0].(File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OpenFile indicates an expected call of OpenFile.
func (mr *MockFSMockRecorder) OpenFile(name, flag, perm any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OpenFile", reflect.TypeOf((*MockFS)(nil).OpenFile), name, flag, perm)
}

// Readlink mocks base method.
func (m *MockFS) Readlink(name string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Readlink", name)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Readlink indicates an expected call of Readlink.
func (mr *MockFSMockRecorder) Readlink(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Readlink", reflect.TypeOf((*MockFS)(nil).Readlink), name)
}

// Remove mocks base method.
func (m *MockFS) Remove(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockFSMockRecorder) Remove(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockFS)(nil).Remove), name)
}

// RemoveAll mocks base method.
func (m *MockFS) RemoveAll(path string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveAll", path)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveAll indicates an expected call of RemoveAll.
func (mr *MockFSMockRecorder) RemoveAll(path any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveAll", reflect.TypeOf((*MockFS)(nil).RemoveAll), path)
}

// Rename mocks base method.
func (m *MockFS) Rename(oldname, newname string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rename", oldname, newname)
	ret0, _ := ret[0].(error)
	return ret0
}

// Rename indicates an expected call of Rename.
func (mr *MockFSMockRecorder) Rename(oldname, newname any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rename", reflect.TypeOf((*MockFS)(nil).Rename), oldname, newname)
}

// Stat mocks base method.
func (m *MockFS) Stat(name string) (fs.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat", name)
	ret0, _ := ret[0].(fs.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stat indicates an expected call of Stat.
func (mr *MockFSMockRecorder) Stat(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockFS)(nil).Stat), name)
}

// Symlink mocks base method.
func (m *MockFS) Symlink(oldname, newname string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Symlink", oldname, newname)
	ret0, _ := ret[0].(error)
	return ret0
}

// Symlink indicates an expected call of Symlink.
func (mr *MockFSMockRecorder) Symlink(oldname, newname any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Symlink", reflect.TypeOf((*MockFS)(nil).Symlink), oldname, newname)
}

// MockSymlinker is a mock of Symlinker interface.
type MockSymlinker struct {
	ctrl     *gomock.Controller
	recorder *MockSymlinkerMockRecorder
}

// MockSymlinkerMockRecorder is the mock recorder for MockSymlinker.
type MockSymlinkerMockRecorder struct {
	mock *MockSymlinker
}

// NewMockSymlinker creates a new mock instance.
func NewMockSymlinker(ctrl *gomock.Controller) *MockSymlinker {
	mock := &MockSymlinker{ctrl: ctrl}
	mock.recorder = &MockSymlinkerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSymlinker) EXPECT() *MockSymlinkerMockRecorder {
	return m.recorder
}

// Lchown mocks base method.
func (m *MockSymlinker) Lchown(name string, uid, gid int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lchown", name, uid, gid)
	ret0, _ := ret[0].(error)
	return ret0
}

// Lchown indicates an expected call of Lchown.
func (mr *MockSymlinkerMockRecorder) Lchown(name, uid, gid any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lchown", reflect.TypeOf((*MockSymlinker)(nil).Lchown), name, uid, gid)
}

// Lstat mocks base method.
func (m *MockSymlinker) Lstat(name string) (fs.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lstat", name)
	ret0, _ := ret[0].(fs.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Lstat indicates an expected call of Lstat.
func (mr *MockSymlinkerMockRecorder) Lstat(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lstat", reflect.TypeOf((*MockSymlinker)(nil).Lstat), name)
}

// Readlink mocks base method.
func (m *MockSymlinker) Readlink(name string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Readlink", name)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Readlink indicates an expected call of Readlink.
func (mr *MockSymlinkerMockRecorder) Readlink(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Readlink", reflect.TypeOf((*MockSymlinker)(nil).Readlink), name)
}

// Symlink mocks base method.
func (m *MockSymlinker) Symlink(oldname, newname string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Symlink", oldname, newname)
	ret0, _ := ret[0].(error)
	return ret0
}

// Symlink indicates an expected call of Symlink.
func (mr *MockSymlinkerMockRecorder) Symlink(oldname, newname any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Symlink", reflect.TypeOf((*MockSymlinker)(nil).Symlink), oldname, newname)
}

// MockFile is a mock of File interface.
type MockFile struct {
	ctrl     *gomock.Controller
	recorder *MockFileMockRecorder
}

// MockFileMockRecorder is the mock recorder for MockFile.
type MockFileMockRecorder struct {
	mock *MockFile
}

// NewMockFile creates a new mock instance.
func NewMockFile(ctrl *gomock.Controller) *MockFile {
	mock := &MockFile{ctrl: ctrl}
	mock.recorder = &MockFileMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFile) EXPECT() *MockFileMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockFile) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockFileMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockFile)(nil).Close))
}

// Name mocks base method.
func (m *MockFile) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockFileMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockFile)(nil).Name))
}

// Read mocks base method.
func (m *MockFile) Read(arg0 []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", arg0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockFileMockRecorder) Read(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockFile)(nil).Read), arg0)
}

// ReadAt mocks base method.
func (m *MockFile) ReadAt(p []byte, off int64) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadAt", p, off)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadAt indicates an expected call of ReadAt.
func (mr *MockFileMockRecorder) ReadAt(p, off any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadAt", reflect.TypeOf((*MockFile)(nil).ReadAt), p, off)
}

// Readdir mocks base method.
func (m *MockFile) Readdir(count int) ([]fs.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Readdir", count)
	ret0, _ := ret[0].([]fs.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Readdir indicates an expected call of Readdir.
func (mr *MockFileMockRecorder) Readdir(count any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Readdir", reflect.TypeOf((*MockFile)(nil).Readdir), count)
}

// Readdirnames mocks base method.
func (m *MockFile) Readdirnames(n int) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Readdirnames", n)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Readdirnames indicates an expected call of Readdirnames.
func (mr *MockFileMockRecorder) Readdirnames(n any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Readdirnames", reflect.TypeOf((*MockFile)(nil).Readdirnames), n)
}

// Seek mocks base method.
func (m *MockFile) Seek(offset int64, whence int) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Seek", offset, whence)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Seek indicates an expected call of Seek.
func (mr *MockFileMockRecorder) Seek(offset, whence any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Seek", reflect.TypeOf((*MockFile)(nil).Seek), offset, whence)
}

// Stat mocks base method.
func (m *MockFile) Stat() (fs.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat")
	ret0, _ := ret[0].(fs.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stat indicates an expected call of Stat.
func (mr *MockFileMockRecorder) Stat() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockFile)(nil).Stat))
}

// Sync mocks base method.
func (m *MockFile) Sync() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sync")
	ret0, _ := ret[0].(error)
	return ret0
}

// Sync indicates an expected call of Sync.
func (mr *MockFileMockRecorder) Sync() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sync", reflect.TypeOf((*MockFile)(nil).Sync))
}

// Truncate mocks base method.
func (m *MockFile) Truncate(size int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Truncate", size)
	ret0, _ := ret[0].(error)
	return ret0
}

// Truncate indicates an expected call of Truncate.
func (mr *MockFileMockRecorder) Truncate(size any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Truncate", reflect.TypeOf((*MockFile)(nil).Truncate), size)
}

// Write mocks base method.
func (m *MockFile) Write(p []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Write", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Write indicates an expected call of Write.
func (mr *MockFileMockRecorder) Write(p any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Write", reflect.TypeOf((*MockFile)(nil).Write), p)
}

// WriteAt mocks base method.
func (m *MockFile) WriteAt(p []byte, off int64) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteAt", p, off)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WriteAt indicates an expected call of WriteAt.
func (mr *MockFileMockRecorder) WriteAt(p, off any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteAt", reflect.TypeOf((*MockFile)(nil).WriteAt), p, off)
}

// WriteString mocks base method.
func (m *MockFile) WriteString(s string) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteString", s)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WriteString indicates an expected call of WriteString.
func (mr *MockFileMockRecorder) WriteString(s any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteString", reflect.TypeOf((*MockFile)(nil).WriteString), s)
}