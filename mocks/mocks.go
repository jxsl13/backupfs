package mocks

import (
	os "os"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	"github.com/jxsl13/backupfs"
)

// MockFs is a mock of Fs interface.
type MockFs struct {
	ctrl     *gomock.Controller
	recorder *MockFsMockRecorder
}

// MockFsMockRecorder is the mock recorder for MockFs.
type MockFsMockRecorder struct {
	mock *MockFs
}

// NewMockFs creates a new mock instance.
func NewMockFs(ctrl *gomock.Controller) *MockFs {
	mock := &MockFs{ctrl: ctrl}
	mock.recorder = &MockFsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFs) EXPECT() *MockFsMockRecorder {
	return m.recorder
}

// Chmod mocks base method.
func (m *MockFs) Chmod(name string, mode os.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chmod", name, mode)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chmod indicates an expected call of Chmod.
func (mr *MockFsMockRecorder) Chmod(name, mode interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chmod", reflect.TypeOf((*MockFs)(nil).Chmod), name, mode)
}

// Chown mocks base method.
func (m *MockFs) Chown(name string, uid, gid int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chown", name, uid, gid)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chown indicates an expected call of Chown.
func (mr *MockFsMockRecorder) Chown(name, uid, gid interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chown", reflect.TypeOf((*MockFs)(nil).Chown), name, uid, gid)
}

// Chtimes mocks base method.
func (m *MockFs) Chtimes(name string, atime, mtime time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chtimes", name, atime, mtime)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chtimes indicates an expected call of Chtimes.
func (mr *MockFsMockRecorder) Chtimes(name, atime, mtime interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chtimes", reflect.TypeOf((*MockFs)(nil).Chtimes), name, atime, mtime)
}

// Create mocks base method.
func (m *MockFs) Create(name string) (backupfs.File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", name)
	ret0, _ := ret[0].(backupfs.File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockFsMockRecorder) Create(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockFs)(nil).Create), name)
}

// Mkdir mocks base method.
func (m *MockFs) Mkdir(name string, perm os.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mkdir", name, perm)
	ret0, _ := ret[0].(error)
	return ret0
}

// Mkdir indicates an expected call of Mkdir.
func (mr *MockFsMockRecorder) Mkdir(name, perm interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mkdir", reflect.TypeOf((*MockFs)(nil).Mkdir), name, perm)
}

// MkdirAll mocks base method.
func (m *MockFs) MkdirAll(path string, perm os.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MkdirAll", path, perm)
	ret0, _ := ret[0].(error)
	return ret0
}

// MkdirAll indicates an expected call of MkdirAll.
func (mr *MockFsMockRecorder) MkdirAll(path, perm interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MkdirAll", reflect.TypeOf((*MockFs)(nil).MkdirAll), path, perm)
}

// Name mocks base method.
func (m *MockFs) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockFsMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockFs)(nil).Name))
}

// Open mocks base method.
func (m *MockFs) Open(name string) (backupfs.File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Open", name)
	ret0, _ := ret[0].(backupfs.File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Open indicates an expected call of Open.
func (mr *MockFsMockRecorder) Open(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Open", reflect.TypeOf((*MockFs)(nil).Open), name)
}

// OpenFile mocks base method.
func (m *MockFs) OpenFile(name string, flag int, perm os.FileMode) (backupfs.File, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OpenFile", name, flag, perm)
	ret0, _ := ret[0].(backupfs.File)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OpenFile indicates an expected call of OpenFile.
func (mr *MockFsMockRecorder) OpenFile(name, flag, perm interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OpenFile", reflect.TypeOf((*MockFs)(nil).OpenFile), name, flag, perm)
}

// Remove mocks base method.
func (m *MockFs) Remove(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockFsMockRecorder) Remove(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockFs)(nil).Remove), name)
}

// RemoveAll mocks base method.
func (m *MockFs) RemoveAll(path string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveAll", path)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveAll indicates an expected call of RemoveAll.
func (mr *MockFsMockRecorder) RemoveAll(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveAll", reflect.TypeOf((*MockFs)(nil).RemoveAll), path)
}

// Rename mocks base method.
func (m *MockFs) Rename(oldname, newname string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rename", oldname, newname)
	ret0, _ := ret[0].(error)
	return ret0
}

// Rename indicates an expected call of Rename.
func (mr *MockFsMockRecorder) Rename(oldname, newname interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rename", reflect.TypeOf((*MockFs)(nil).Rename), oldname, newname)
}

// Stat mocks base method.
func (m *MockFs) Stat(name string) (os.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat", name)
	ret0, _ := ret[0].(os.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stat indicates an expected call of Stat.
func (mr *MockFsMockRecorder) Stat(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockFs)(nil).Stat), name)
}
