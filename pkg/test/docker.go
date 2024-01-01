package test

import (
	"sync"

	dockerclient "github.com/fsouza/go-dockerclient"

	"github.com/openshift/source-to-image/pkg/docker"
)

// FakeDocker provides a fake docker interface
type FakeDocker struct {
	LocalRegistryImage           string
	LocalRegistryResult          bool
	LocalRegistryError           error
	RemoveContainerID            string
	RemoveContainerError         error
	DefaultURLImage              string
	DefaultURLResult             string
	DefaultURLError              error
	RunContainerOpts             docker.RunContainerOptions
	RunContainerError            error
	RunContainerErrorBeforeStart bool
	RunContainerContainerID      string
	RunContainerCmd              []string
	GetImageIDImage              string
	GetImageIDResult             string
	GetImageIDError              error
	GetImageUserImage            string
	GetImageUserResult           string
	GetImageUserError            error
	CommitContainerOpts          docker.CommitContainerOptions
	CommitContainerResult        string
	CommitContainerError         error
	RemoveImageName              string
	RemoveImageError             error
	BuildImageOpts               docker.BuildImageOptions
	BuildImageError              error
	PullResult                   bool
	PullError                    error
	OnBuildImage                 string
	OnBuildResult                []string
	OnBuildError                 error
	IsOnBuildResult              bool
	IsOnBuildImage               string
	Labels                       map[string]string
	LabelsError                  error

	mutex sync.Mutex
}

// IsImageInLocalRegistry checks if the image exists in the fake local registry
func (f *FakeDocker) IsImageInLocalRegistry(imageName string) (bool, error) {
	f.LocalRegistryImage = imageName
	return f.LocalRegistryResult, f.LocalRegistryError
}

// IsImageOnBuild  returns true if the builder has onbuild instructions
func (f *FakeDocker) IsImageOnBuild(imageName string) bool {
	f.IsOnBuildImage = imageName
	return f.IsOnBuildResult
}

// GetOnBuild returns the list of onbuild instructions for the given image
func (f *FakeDocker) GetOnBuild(imageName string) ([]string, error) {
	f.OnBuildImage = imageName
	return f.OnBuildResult, f.OnBuildError
}

// RemoveContainer removes a fake Docker container
func (f *FakeDocker) RemoveContainer(id string) error {
	f.RemoveContainerID = id
	return f.RemoveContainerError
}

// GetScriptsURL returns a default STI scripts URL
func (f *FakeDocker) GetScriptsURL(image string) (string, error) {
	f.DefaultURLImage = image
	return f.DefaultURLResult, f.DefaultURLError
}

// RunContainer runs a fake Docker container
func (f *FakeDocker) RunContainer(opts docker.RunContainerOptions) error {
	f.RunContainerOpts = opts
	if f.RunContainerErrorBeforeStart {
		return f.RunContainerError
	}
	if opts.OnStart != nil {
		if err := opts.OnStart(); err != nil {
			return err
		}
	}
	if opts.PostExec != nil {
		opts.PostExec.PostExecute(f.RunContainerContainerID, string(opts.Command))
	}
	return f.RunContainerError
}

// GetImageID returns a fake Docker image ID
func (f *FakeDocker) GetImageID(image string) (string, error) {
	f.GetImageIDImage = image
	return f.GetImageIDResult, f.GetImageIDError
}

// GetImageUser returns a fake user
func (f *FakeDocker) GetImageUser(image string) (string, error) {
	f.GetImageUserImage = image
	return f.GetImageUserResult, f.GetImageUserError
}

// CommitContainer commits a fake Docker container
func (f *FakeDocker) CommitContainer(opts docker.CommitContainerOptions) (string, error) {
	f.CommitContainerOpts = opts
	return f.CommitContainerResult, f.CommitContainerError
}

// RemoveImage removes a fake Docker image
func (f *FakeDocker) RemoveImage(name string) error {
	f.RemoveImageName = name
	return f.RemoveImageError
}

// CheckImage checks image in local registry
func (f *FakeDocker) CheckImage(name string) (*dockerclient.Image, error) {
	return nil, nil
}

// PullImage pulls a fake docker image
func (f *FakeDocker) PullImage(imageName string) (*dockerclient.Image, error) {
	if f.PullResult {
		return &dockerclient.Image{}, nil
	}
	return nil, f.PullError
}

// CheckAndPullImage pulls a fake docker image
func (f *FakeDocker) CheckAndPullImage(name string) (*dockerclient.Image, error) {
	return nil, nil
}

// BuildImage builds image
func (f *FakeDocker) BuildImage(opts docker.BuildImageOptions) error {
	f.BuildImageOpts = opts
	return f.BuildImageError
}

func (f *FakeDocker) GetLabels(name string) (map[string]string, error) {
	return f.Labels, f.LabelsError
}
