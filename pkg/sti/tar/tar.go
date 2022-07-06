package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/errors"
)

// defaultTimeout is the amount of time that the untar will wait for a tar
// stream to extract a single file. A timeout is needed to guard against broken
// connections in which it would wait for a long time to untar and nothing would happen
const defaultTimeout = 5 * time.Second

// defaultExclusionPattern is the pattern of files that will not be included in a tar
// file when creating one. By default it is any file inside a .git metadata directory
var defaultExclusionPattern = regexp.MustCompile("((^\\.git\\/)|(\\/.git\\/)|(\\/.git$))")

// Tar can create and extract tar files used in an STI build
type Tar interface {
	// CreateTarFile creates a tar file in the base directory
	// using the contents of dir directory
	// The name of the new tar file is returned if successful
	CreateTarFile(base, dir string) (string, error)

	// ExtractTarStream extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout
	ExtractTarStream(dir string, reader io.Reader) error
}

// NewTar creates a new Tar
func NewTar() Tar {
	return &stiTar{
		exclude: defaultExclusionPattern,
		timeout: defaultTimeout,
	}
}

// stiTar is an implementation of the Tar interface
type stiTar struct {
	timeout time.Duration
	exclude *regexp.Regexp
}

// CreateTarFile creates a tar file from the given directory
// while excluding files that match the given exclusion pattern
// It returns the name of the created file
func (t *stiTar) CreateTarFile(base, dir string) (string, error) {
	tarFile, err := ioutil.TempFile(base, "tar")
	defer tarFile.Close()
	if err != nil {
		return "", err
	}
	if err = t.CreateTarStream(dir, tarFile); err != nil {
		return "", err
	}
	return tarFile.Name(), nil
}

func (t *stiTar) shouldExclude(path string) bool {
	return t.exclude != nil && t.exclude.MatchString(path)
}

// CreateTarStream creates a tar stream on the given writer from
// the given directory while excluding files that match the given
// exclusion pattern.
func (t *stiTar) CreateTarStream(dir string, writer io.Writer) error {
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && !t.shouldExclude(path) {
			// if file is a link just writing header info is enough
			if info.Mode()&os.ModeSymlink != 0 {
				if err := t.writeTarHeader(tarWriter, dir, path, info); err != nil {
					glog.Errorf("	Error writing header for %s: %v", info.Name(), err)
				}
				return nil
			}

			// regular files are copied into tar, if accessible
			file, err := os.Open(path)
			if err != nil {
				glog.Errorf("Ignoring file %s: %v", path, err)
				return nil
			}
			defer file.Close()
			if err := t.writeTarHeader(tarWriter, dir, path, info); err != nil {
				glog.Errorf("Error writing header for %s: %v", info.Name(), err)
				return nil
			}
			if _, err = io.Copy(tarWriter, file); err != nil {
				glog.Errorf("Error copying file %s to tar: %v", path, err)
				return err
			}
		}
		return nil
	})

	if err != nil {
		glog.Errorf("Error writing tar: %v", err)
		return err
	}

	return nil
}

// writeTarHeader writes tar header for given file, returns error if operation fails
func (t *stiTar) writeTarHeader(tarWriter *tar.Writer, dir string, path string, info os.FileInfo) error {
	var (
		link string
		err  error
	)
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return err
		}
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = path[1+len(dir):]
	glog.V(3).Infof("Adding to tar: %s as %s", path, header.Name)
	if err = tarWriter.WriteHeader(header); err != nil {
		return err
	}

	return nil
}

// ExtractTarStream extracts files from a given tar stream.
// Times out if reading from the stream for any given file
// exceeds the value of timeout
func (t *stiTar) ExtractTarStream(dir string, reader io.Reader) error {
	tarReader := tar.NewReader(reader)
	errorChannel := make(chan error)
	timeout := t.timeout
	timeoutTimer := time.NewTimer(timeout)
	go func() {
		for {
			header, err := tarReader.Next()
			timeoutTimer.Reset(timeout)
			if err == io.EOF {
				errorChannel <- nil
				break
			}
			if err != nil {
				glog.Errorf("Error reading next tar header: %v", err)
				errorChannel <- err
				break
			}
			if header.FileInfo().IsDir() {
				dirPath := filepath.Join(dir, header.Name)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %s: %v", dirPath, err)
					errorChannel <- err
					break
				}
			} else {
				fileDir := filepath.Dir(header.Name)
				dirPath := filepath.Join(dir, fileDir)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %s: %v", dirPath, err)
					errorChannel <- err
					break
				}
				path := filepath.Join(dir, header.Name)
				glog.V(3).Infof("Creating %s", path)
				success := false
				// The file times need to be modified after it's been closed
				// thus this function is deferred before the file close
				defer func() {
					if success && os.Chtimes(path, time.Now(),
						header.FileInfo().ModTime()) != nil {
						glog.Errorf("Error setting file dates: %v", err)
						errorChannel <- err
					}
				}()
				file, err := os.Create(path)
				defer file.Close()
				if err != nil {
					glog.Errorf("Error creating file %s: %v", path, err)
					errorChannel <- err
					break
				}
				glog.V(3).Infof("Extracting/writing %s", path)
				written, err := io.Copy(file, tarReader)
				if err != nil {
					glog.Errorf("Error writing file: %v", err)
					errorChannel <- err
					break
				}
				if written != header.Size {
					message := fmt.Sprintf("Wrote %d bytes, expected to write %d",
						written, header.Size)
					glog.Errorf(message)
					errorChannel <- fmt.Errorf(message)
					break
				}
				if err = file.Chmod(header.FileInfo().Mode()); err != nil {
					glog.Errorf("Error setting file mode: %v", err)
					errorChannel <- err
					break
				}
				glog.V(3).Infof("Done with %s", path)
				success = true
			}
		}
	}()

	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				glog.Errorf("Error extracting tar stream")
			} else {
				glog.V(2).Infof("Done extracting tar stream")
			}
			return err
		case <-timeoutTimer.C:
			return errors.NewTarTimeoutError()
		}
	}
}
