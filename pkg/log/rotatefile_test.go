package log

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var fakeCurrentTime = time.Now()

func fakeTime() time.Time {
	return fakeCurrentTime
}

func TestNewFile(t *testing.T) {
	require := require.New(t)
	currentTime = fakeTime
	dir := makeTempDir("TestNewFile", t)
	defer os.RemoveAll(dir)
	filename := logFile(dir)
	l := &RotateFile{
		Filename: filename,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(err)
	require.Equal(len(b), n)
	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	//test maxbackups = 0, can be triggered manually
	newFakeTime()

	err = l.Rotate()
	require.NoError(err)

	<-time.After(10 * time.Millisecond)

	filename2 := backupFile(dir)
	existsWithContent(filename2, b, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)

}

func TestRotate(t *testing.T) {
	require := require.New(t)
	currentTime = fakeTime
	dir := makeTempDir("TestRotate", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	l := &RotateFile{
		Filename:   filename,
		MaxBackups: 1,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(err)
	require.Equal(len(b), n)
	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	err = l.Rotate()
	require.NoError(err)

	<-time.After(10 * time.Millisecond)

	fileCount(dir, 2, t)

	filename2 := backupFile(dir)
	existsWithContent(filename2, b, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)

	newFakeTime()

	err = l.Rotate()
	require.NoError(err)

	<-time.After(10 * time.Millisecond)

	filename3 := backupFile(dir)
	existsWithContent(filename3, []byte{}, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	require.NoError(err)
	require.Equal(len(b2), n)
	existsWithContent(filename, b2, t)

}

func makeTempDir(name string, t testing.TB) string {
	require := require.New(t)
	dir := filepath.Join(os.TempDir(), name)
	require.NoError(os.Mkdir(dir, 0700))
	return dir
}

func logFile(dir string) string {
	return filepath.Join(dir, "foobar.log")
}

func existsWithContent(path string, content []byte, t testing.TB) {
	require := require.New(t)
	info, err := os.Stat(path)
	require.NoError(err)
	require.Equal(int64(len(content)), info.Size())

	b, err := ioutil.ReadFile(path)
	require.Nil(err)
	require.Equal(content, b)
}

func fileCount(dir string, exp int, t testing.TB) {
	require := require.New(t)
	files, err := ioutil.ReadDir(dir)
	require.NoError(err)
	require.Equal(exp, len(files))
}

// newFakeTime sets the fake "current time" to two days later.
func newFakeTime() {
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24 * 2)
}

func backupFile(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().UTC().Format(defaultBackupTimeFormat)+".log."+strconv.FormatInt(fakeTime().Unix(), 10))
}
