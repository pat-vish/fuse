// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package hellofs_test

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/gcsfuse/timeutil"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
	"golang.org/x/net/context"
)

func TestHelloFS(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// Boilerplate
////////////////////////////////////////////////////////////////////////

type HelloFSTest struct {
	clock timeutil.SimulatedClock
	mfs   *fuse.MountedFileSystem
}

var _ SetUpInterface = &HelloFSTest{}
var _ TearDownInterface = &HelloFSTest{}

func init() { RegisterTestSuite(&HelloFSTest{}) }

func (t *HelloFSTest) SetUp(ti *TestInfo) {
	var err error

	// Set up a fixed, non-zero time.
	t.clock.SetTime(time.Now())

	// Set up a temporary directory for mounting.
	mountPoint, err := ioutil.TempDir("", "hello_fs_test")
	if err != nil {
		panic("ioutil.TempDir: " + err.Error())
	}

	// Mount a file system.
	fs := &samples.HelloFS{
		Clock: &t.clock,
	}

	if t.mfs, err = fuse.Mount(mountPoint, fs); err != nil {
		panic("Mount: " + err.Error())
	}

	if err = t.mfs.WaitForReady(context.Background()); err != nil {
		panic("MountedFileSystem.WaitForReady: " + err.Error())
	}
}

func (t *HelloFSTest) TearDown() {
	// Unmount the file system. Try again on "resource busy" errors.
	delay := 10 * time.Millisecond
	for {
		err := t.mfs.Unmount()
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "resource busy") {
			log.Println("Resource busy error while unmounting; trying again")
			time.Sleep(delay)
			delay = time.Duration(1.3 * float64(delay))
			continue
		}

		panic("MountedFileSystem.Unmount: " + err.Error())
	}

	if err := t.mfs.Join(context.Background()); err != nil {
		panic("MountedFileSystem.Join: " + err.Error())
	}
}

////////////////////////////////////////////////////////////////////////
// Test functions
////////////////////////////////////////////////////////////////////////

func (t *HelloFSTest) ReadDir_Root() {
	entries, err := ioutil.ReadDir(t.mfs.Dir())

	AssertEq(nil, err)
	AssertEq(2, len(entries))
	var fi os.FileInfo

	// dir
	fi = entries[0]
	ExpectEq("dir", fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(os.ModeDir|0500, fi.Mode())
	ExpectEq(0, t.clock.Now().Sub(fi.ModTime()), "ModTime: %v", fi.ModTime())
	ExpectTrue(fi.IsDir())

	// hello
	fi = entries[1]
	ExpectEq("hello", fi.Name())
	ExpectEq(len("Hello, world!"), fi.Size())
	ExpectEq(0400, fi.Mode())
	ExpectEq(0, t.clock.Now().Sub(fi.ModTime()), "ModTime: %v", fi.ModTime())
	ExpectFalse(fi.IsDir())
}

func (t *HelloFSTest) ReadDir_Dir() {
	entries, err := ioutil.ReadDir(path.Join(t.mfs.Dir(), "dir"))

	AssertEq(nil, err)
	AssertEq(1, len(entries))
	var fi os.FileInfo

	// world
	fi = entries[0]
	ExpectEq("world", fi.Name())
	ExpectEq(len("Hello, world!"), fi.Size())
	ExpectEq(0400, fi.Mode())
	ExpectEq(0, t.clock.Now().Sub(fi.ModTime()), "ModTime: %v", fi.ModTime())
	ExpectFalse(fi.IsDir())
}

func (t *HelloFSTest) ReadDir_NonExistent() {
	_, err := ioutil.ReadDir(path.Join(t.mfs.Dir(), "foobar"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file")))
}

func (t *HelloFSTest) Stat_Hello() {
	fi, err := os.Stat(path.Join(t.mfs.Dir(), "hello"))
	AssertEq(nil, err)

	ExpectEq("hello", fi.Name())
	ExpectEq(len("Hello, world!"), fi.Size())
	ExpectEq(0400, fi.Mode())
	ExpectEq(0, t.clock.Now().Sub(fi.ModTime()), "ModTime: %v", fi.ModTime())
	ExpectFalse(fi.IsDir())
}

func (t *HelloFSTest) Stat_Dir() {
	fi, err := os.Stat(path.Join(t.mfs.Dir(), "dir"))
	AssertEq(nil, err)

	ExpectEq("dir", fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(0500|os.ModeDir, fi.Mode())
	ExpectEq(0, t.clock.Now().Sub(fi.ModTime()), "ModTime: %v", fi.ModTime())
	ExpectTrue(fi.IsDir())
}

func (t *HelloFSTest) Stat_World() {
	fi, err := os.Stat(path.Join(t.mfs.Dir(), "dir/world"))
	AssertEq(nil, err)

	ExpectEq("world", fi.Name())
	ExpectEq(len("Hello, world!"), fi.Size())
	ExpectEq(0400, fi.Mode())
	ExpectEq(0, t.clock.Now().Sub(fi.ModTime()), "ModTime: %v", fi.ModTime())
	ExpectFalse(fi.IsDir())
}

func (t *HelloFSTest) Stat_NonExistent() {
	_, err := os.Stat(path.Join(t.mfs.Dir(), "foobar"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file")))
}

func (t *HelloFSTest) ReadFile_Hello() {
	slice, err := ioutil.ReadFile(path.Join(t.mfs.Dir(), "hello"))

	AssertEq(nil, err)
	ExpectEq("Hello, world!", string(slice))
}

func (t *HelloFSTest) ReadFile_Dir() {
	_, err := ioutil.ReadFile(path.Join(t.mfs.Dir(), "dir"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("is a directory")))
}

func (t *HelloFSTest) ReadFile_World() {
	slice, err := ioutil.ReadFile(path.Join(t.mfs.Dir(), "dir/world"))

	AssertEq(nil, err)
	ExpectEq("Hello, world!", string(slice))
}

func (t *HelloFSTest) OpenAndRead() {
	var buf []byte = make([]byte, 1024)
	var n int
	var off int64
	var err error

	// Open the file.
	f, err := os.Open(path.Join(t.mfs.Dir(), "hello"))
	defer func() {
		if f != nil {
			ExpectEq(nil, f.Close())
		}
	}()

	AssertEq(nil, err)

	// Seeking shouldn't affect the random access reads below.
	_, err = f.Seek(7, 0)
	AssertEq(nil, err)

	// Random access reads
	n, err = f.ReadAt(buf[:2], 0)
	AssertEq(nil, err)
	ExpectEq(2, n)
	ExpectEq("He", string(buf[:n]))

	n, err = f.ReadAt(buf[:2], int64(len("Hel")))
	AssertEq(nil, err)
	ExpectEq(2, n)
	ExpectEq("lo", string(buf[:n]))

	n, err = f.ReadAt(buf[:3], int64(len("Hello, wo")))
	AssertEq(nil, err)
	ExpectEq(3, n)
	ExpectEq("rld", string(buf[:n]))

	// Read beyond end.
	n, err = f.ReadAt(buf[:3], int64(len("Hello, world")))
	AssertEq(io.EOF, err)
	ExpectEq(1, n)
	ExpectEq("!", string(buf[:n]))

	// Seek then read the rest.
	off, err = f.Seek(int64(len("Hel")), 0)
	AssertEq(nil, err)
	AssertEq(len("Hel"), off)

	n, err = io.ReadFull(f, buf[:len("lo, world!")])
	AssertEq(nil, err)
	ExpectEq(len("lo, world!"), n)
	ExpectEq("lo, world!", string(buf[:n]))
}

func (t *HelloFSTest) Open_NonExistent() {
	_, err := os.Open(path.Join(t.mfs.Dir(), "foobar"))

	AssertNe(nil, err)
	ExpectThat(err, Error(HasSubstr("no such file")))
}