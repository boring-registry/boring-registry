package storage

import (
	"bytes"
	"context"
	"io"
	iofs "io/fs"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/stretchr/testify/assert"
)

type localStorageAttrsForTest struct {
	server              FileServer
	storageDir          string
	moduleArchiveFormat string
}

type mockLocalFileInfo struct {
	isDir bool
}

func (ml *mockLocalFileInfo) Name() string {
	return ""
}

func (ml *mockLocalFileInfo) Size() int64 {
	return 0
}

func (ml *mockLocalFileInfo) Mode() iofs.FileMode {
	return iofs.ModeAppend
}

func (ml *mockLocalFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (ml *mockLocalFileInfo) IsDir() bool {
	return ml.isDir
}

func (ml *mockLocalFileInfo) Sys() any {
	return nil
}

type mockFile struct {
	buf *bytes.Buffer
}

func (mf *mockFile) Read(p []byte) (n int, err error) {
	return mf.buf.Read(p)
}

func (mf *mockFile) Write(p []byte) (n int, err error) {
	return mf.buf.Write(p)
}

func (mf *mockFile) Close() error {
	return nil
}

type mockLocalFileSystem struct {
	files     map[string]io.ReadWriteCloser
	fileStats map[string]os.FileInfo
	dirs      map[string][]os.DirEntry
	mkdirErr  bool
}

func (fs mockLocalFileSystem) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if file, exist := fs.files[name]; !exist {
		return nil, errors.New("file not exist")
	} else {
		return file, nil
	}
}

func (fs mockLocalFileSystem) ReadFile(name string) ([]byte, error) {
	file, exist := fs.files[name]
	if !exist {
		return nil, errors.New("file not exist")
	}

	return ioutil.ReadAll(file)
}

func (fs mockLocalFileSystem) Stat(name string) (os.FileInfo, error) {
	stat, exist := fs.fileStats[name]
	if !exist {
		return nil, errors.New("file not exist")
	}

	return stat, nil
}

func (fs mockLocalFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	dir, exist := fs.dirs[name]
	if !exist {
		return nil, errors.New("dir not exist")
	}

	return dir, nil
}

func (fs mockLocalFileSystem) MkdirAll(name string, perm os.FileMode) error {
	if fs.mkdirErr {
		return errors.New("mkdir failed")
	}

	return nil
}

type mockFileServer struct {
	addr     string
	endpoint string
}

func (m *mockFileServer) ListenAndServe() error {
	return nil
}

func (m *mockFileServer) Addr() string {
	return m.addr
}

func (m *mockFileServer) Endpoint() string {
	return m.endpoint
}

func TestLocalStorage_GetProvider(t *testing.T) {
	type args struct {
		namespace string
		name      string
		version   string
		os_       string
		arch      string
	}

	cases := []struct {
		name      string
		lfs       LocalFileSystem
		attrs     localStorageAttrsForTest
		args      args
		expected  core.Provider
		expectErr bool
	}{
		{
			name: "emtpy namespaces",
			lfs:  nil,
			args: args{
				namespace: "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy name",
			lfs:  nil,
			args: args{
				namespace: "xxx",
				name:      "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy version",
			lfs:  nil,
			args: args{
				namespace: "xxx",
				name:      "xxx",
				version:   "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy os",
			lfs:  nil,
			args: args{
				namespace: "xxx",
				name:      "xxx",
				version:   "xxx",
				os_:       "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy arch",
			lfs:  nil,
			args: args{
				namespace: "xxx",
				name:      "xxx",
				version:   "xxx",
				os_:       "xxx",
				arch:      "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "provider not exist in local file system",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			lfs: mockLocalFileSystem{},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "shasum not exist in local file system",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			lfs: mockLocalFileSystem{
				fileStats: map[string]os.FileInfo{
					"/test/providers/a/b/terraform-provider-b_c_d_e.zip": &mockLocalFileInfo{},
				},
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
				server: &fileServer{
					addr:     ":8080",
					endpoint: "localhost:8080",
				},
			},
			expectErr: true,
		},
		{
			name: "shasum sig not exist in local file system",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			lfs: mockLocalFileSystem{
				fileStats: map[string]os.FileInfo{
					"/test/providers/a/b/terraform-provider-b_c_d_e.zip":    &mockLocalFileInfo{},
					"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS": &mockLocalFileInfo{},
				},
				files: map[string]io.ReadWriteCloser{
					"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS": &mockFile{
						buf: bytes.NewBuffer([]byte(`198e1bb88df52eb1201953d0b1e6c4ac48eac2440e395887cb4eca655d68b120  terraform-provider-b_c_d_e.zip`)),
					},
				},
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "get provider success",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			lfs: mockLocalFileSystem{
				fileStats: map[string]os.FileInfo{
					"/test/providers/a/b/terraform-provider-b_c_d_e.zip":        &mockLocalFileInfo{},
					"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS":     &mockLocalFileInfo{},
					"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS.sig": &mockLocalFileInfo{},
					"/test/providers/a/signing-keys.json":                       &mockLocalFileInfo{},
				},
				files: map[string]io.ReadWriteCloser{
					"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS": &mockFile{
						buf: bytes.NewBuffer([]byte(`198e1bb88df52eb1201953d0b1e6c4ac48eac2440e395887cb4eca655d68b120  terraform-provider-b_c_d_e.zip`)),
					},
					"/test/providers/a/signing-keys.json": &mockFile{
						buf: bytes.NewBuffer([]byte(`
                            {
                              "gpg_public_keys": [
                                {
                                  "key_id": "51852D87348FFC4C",
                                  "ascii_armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n..."
                                }
                              ]
                            }`)),
					},
				},
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
				server: &mockFileServer{
					addr:     ":8080",
					endpoint: "localhost",
				},
			},
			expectErr: false,
			expected: core.Provider{
				Namespace:           "a",
				Name:                "b",
				Version:             "c",
				OS:                  "d",
				Arch:                "e",
				Filename:            "terraform-provider-b_c_d_e.zip",
				DownloadURL:         "http://localhost/providers/a/b/terraform-provider-b_c_d_e.zip",
				SHASum:              "198e1bb88df52eb1201953d0b1e6c4ac48eac2440e395887cb4eca655d68b120",
				SHASumsURL:          "http://localhost/providers/a/b/terraform-provider-b_c_SHA256SUMS",
				SHASumsSignatureURL: "http://localhost/providers/a/b/terraform-provider-b_c_SHA256SUMS.sig",
				SigningKeys: core.SigningKeys{
					GPGPublicKeys: []core.GPGPublicKey{
						{
							KeyID:      "51852D87348FFC4C",
							ASCIIArmor: "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n...",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ls := NewLocalStorage(tc.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.moduleArchiveFormat)
			p, err := ls.GetProvider(
				context.Background(),
				tc.args.namespace,
				tc.args.name,
				tc.args.version,
				tc.args.os_,
				tc.args.arch,
			)

			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, p)
			}
		})
	}
}
