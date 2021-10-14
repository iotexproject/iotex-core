package log

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	_location = time.UTC
)

type RotateFile struct {
	// Filename is the file to write logs to.  Backup log files will be retained
	// in the same directory.
	// os.TempDir() if empty.
	Filename string `json:"filename" yaml:"filename"`
	// MaxBackups is the maximum number of old log files to retain.  The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	MaxBackups int `json:"maxbackups" yaml:"maxbackups"`

	// BackupTimeFormat determines if the time used for formatting the backup file name
	BackupTimeFormat string `json:"backupTimeFormat" yaml:"backupTimeFormat"`

	file              *os.File
	currentBackupName string
	mu                sync.Mutex
}

// filename generates the name of the logfile from the current time.
func (f *RotateFile) filename() string {
	if f.Filename != "" {
		return f.Filename
	}
	name := filepath.Base(os.Args[0]) + ".log"
	return filepath.Join(os.TempDir(), name)
}

// dir returns the directory for the current filename.
func (f *RotateFile) dir() string {
	return filepath.Dir(f.filename())
}

func (f *RotateFile) open() error {
	var err error
	t := time.Now().In(_location)
	f.currentBackupName = t.Format(f.BackupTimeFormat)

	f.file, err = os.OpenFile(f.filename(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	return err
}

// Close implements io.Closer, and closes the current logfile.
func (f *RotateFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.close()
}

// close closes the file if it is open.
func (f *RotateFile) close() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

// rotate on new day
func (f *RotateFile) reopenIfNeeded() error {
	if f.file == nil {
		return f.open()
	}
	t := time.Now().In(_location)
	if f.currentBackupName == t.Format(f.BackupTimeFormat) {
		return nil
	}
	err := f.close()
	if err != nil {
		return err
	}
	return f.open()
}

// Write writes data to a file
func (f *RotateFile) Write(d []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	err := f.reopenIfNeeded()
	if err != nil {
		return 0, err
	}

	return f.file.Write(d)
}

// logInfo is a convenience struct to return the filename and its embedded
// timestamp.
type logInfo struct {
	timestamp time.Time
	os.FileInfo
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []logInfo

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}
