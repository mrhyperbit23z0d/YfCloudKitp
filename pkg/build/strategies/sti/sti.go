package sti

import (
	"bufio"
	"io"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/layered"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/git"
	"github.com/openshift/source-to-image/pkg/scripts"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
)

const (
	// maxErrorOutput is the maximum length of the error output saved for processing
	maxErrorOutput = 1024
	// defaultLocation is the default location of the scripts and sources in image
	defaultLocation = "/tmp"
)

var (
	// List of directories that needs to be present inside working dir
	workingDirs = []string{
		api.UploadScripts,
		api.Source,
		api.DefaultScripts,
		api.UserScripts,
	}
)

// STI strategy executes the STI build.
// For more details about STI, visit https://github.com/openshift/source-to-image
type STI struct {
	request          *api.Request
	result           *api.Result
	postExecutor     docker.PostExecutor
	installer        scripts.Installer
	git              git.Git
	fs               util.FileSystem
	tar              tar.Tar
	docker           docker.Docker
	callbackInvoker  util.CallbackInvoker
	requiredScripts  []string
	optionalScripts  []string
	externalScripts  map[string]bool
	installedScripts map[string]bool

	// Interfaces
	preparer  build.Preparer
	artifacts build.IncrementalBuilder
	scripts   build.ScriptsHandler
	source    build.Downloader
	garbage   build.Cleaner
	layered   build.Builder
}

// New returns the instance of STI builder strategy for the given request.
// If the layeredBuilder parameter is specified, then the builder provided will
// be used for the case that the base Docker image does not have 'tar' or 'bash'
// installed.
func New(req *api.Request) (*STI, error) {
	docker, err := docker.New(req.DockerSocket)
	if err != nil {
		return nil, err
	}
	inst := scripts.NewInstaller(req.BaseImage, req.ScriptsURL, docker)

	b := &STI{
		installer:        inst,
		request:          req,
		docker:           docker,
		git:              git.New(),
		fs:               util.NewFileSystem(),
		tar:              tar.New(),
		callbackInvoker:  util.NewCallbackInvoker(),
		requiredScripts:  []string{api.Assemble, api.Run},
		optionalScripts:  []string{api.SaveArtifacts},
		externalScripts:  map[string]bool{},
		installedScripts: map[string]bool{},
	}

	// The sources are downloaded using the GIT downloader.
	// TODO: Add more SCM in future.
	b.source = &git.Clone{b.git, b.fs}
	b.garbage = &build.DefaultCleaner{b.fs, b.docker}
	b.layered, err = layered.New(req, b)

	// Set interfaces
	b.preparer = b
	b.artifacts = b
	b.scripts = b
	b.postExecutor = b
	return b, err
}

// Build processes a Request and returns a *api.Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func (b *STI) Build(request *api.Request) (*api.Result, error) {
	defer b.garbage.Cleanup(request)

	glog.Infof("Building %s", request.Tag)
	if err := b.preparer.Prepare(request); err != nil {
		return nil, err
	}

	if b.request.Incremental = b.artifacts.Exists(request); b.request.Incremental {
		glog.V(1).Infof("Existing image for tag %s detected for incremental build", request.Tag)
	} else {
		glog.V(1).Infof("Clean build will be performed")
	}

	glog.V(2).Infof("Performing source build from %s", request.Source)
	if request.Incremental {
		if err := b.artifacts.Save(request); err != nil {
			glog.Warningf("Error saving previous build artifacts: %v", err)
			glog.Warning("Clean build will be performed!")
		}
	}

	glog.V(1).Infof("Building %s", request.Tag)
	if err := b.scripts.Execute(api.Assemble, request); err != nil {
		switch e := err.(type) {
		case errors.ContainerError:
			if !isMissingRequirements(e.Output) {
				return nil, err
			}
			return b.layered.Build(request)
		default:
			return nil, err
		}
	}

	return b.result, nil
}

// Prepare prepares the source code and tar for build
func (b *STI) Prepare(request *api.Request) error {
	var err error
	if request.WorkingDir, err = b.fs.CreateWorkingDirectory(); err != nil {
		return err
	}

	b.result = &api.Result{
		Success:    false,
		WorkingDir: request.WorkingDir,
	}

	// Setup working directories
	for _, v := range workingDirs {
		if err := b.fs.MkdirAll(filepath.Join(request.WorkingDir, v)); err != nil {
			return err
		}
	}

	// fetch sources, for theirs .sti/bin might contain sti scripts
	if len(request.Source) > 0 {
		if err = b.source.Download(request); err != nil {
			return err
		}
	}

	// get the scripts
	required, err := b.installer.InstallRequired(b.requiredScripts, request.WorkingDir)
	if err != nil {
		return err
	}
	optional := b.installer.InstallOptional(b.optionalScripts, request.WorkingDir)

	for _, r := range append(required, optional...) {
		if r.Error == nil {
			glog.V(1).Infof("Using %v from %s", r.Script, r.URL)
			b.externalScripts[r.Script] = r.Downloaded
			b.installedScripts[r.Script] = r.Installed
		} else {
			glog.Warningf("Error getting %v from %s: %v", r.Script, r.URL, r.Error)
		}
	}

	return nil
}

// SetScripts allows to override default required and optional scripts
func (b *STI) SetScripts(required, optional []string) {
	b.requiredScripts = required
	b.optionalScripts = optional
}

// PostExecute allows to execute post-build actions after the Docker build
// finishes.
func (b *STI) PostExecute(containerID string, location string) error {
	var (
		err             error
		previousImageID string
	)

	if b.request.Incremental && b.request.RemovePreviousImage {
		if previousImageID, err = b.docker.GetImageID(b.request.Tag); err != nil {
			glog.Errorf("Error retrieving previous image's metadata: %v", err)
		}
	}

	cmd := []string{}
	opts := docker.CommitContainerOptions{
		Command:     append(cmd, filepath.Join(location, api.Run)),
		Env:         b.generateConfigEnv(),
		ContainerID: containerID,
		Repository:  b.request.Tag,
	}
	imageID, err := b.docker.CommitContainer(opts)
	if err != nil {
		return errors.NewBuildError(b.request.Tag, err)
	}
	b.result.Success = true
	glog.Infof("Successfully built %s", b.request.Tag)

	b.result.ImageID = imageID
	glog.V(1).Infof("Tagged %s as %s", imageID, b.request.Tag)

	if b.request.Incremental && b.request.RemovePreviousImage && previousImageID != "" {
		glog.V(1).Infof("Removing previously-tagged image %s", previousImageID)
		if err = b.docker.RemoveImage(previousImageID); err != nil {
			glog.Errorf("Unable to remove previous image: %v", err)
		}
	}

	if b.request.CallbackURL != "" {
		b.result.Messages = b.callbackInvoker.ExecuteCallback(b.request.CallbackURL,
			b.result.Success, b.result.Messages)
	}

	return nil
}

// Exists determines if the current build supports incremental workflow.
// It checks if the previous image exists in the system and if so, then it
// verifies that the save-artifacts script is present.
func (b *STI) Exists(request *api.Request) bool {
	if request.Clean {
		return false
	}

	// can only do incremental build if runtime image exists, so always pull image
	previousImageExists, _ := b.docker.IsImageInLocalRegistry(request.Tag)
	if image, _ := b.docker.PullImage(request.Tag); image != nil {
		previousImageExists = true
	}

	return previousImageExists && b.installedScripts[api.SaveArtifacts]
}

// Save extracts and restores the build artifacts from the previous build to a
// current build.
func (b *STI) Save(request *api.Request) (err error) {
	artifactTmpDir := filepath.Join(request.WorkingDir, "upload", "artifacts")
	if err = b.fs.Mkdir(artifactTmpDir); err != nil {
		return err
	}

	image := request.Tag
	reader, writer := io.Pipe()
	glog.V(1).Infof("Saving build artifacts from image %s to path %s", image, artifactTmpDir)
	extractFunc := func() error {
		defer reader.Close()
		return b.tar.ExtractTarStream(artifactTmpDir, reader)
	}

	opts := docker.RunContainerOptions{
		Image:           image,
		ExternalScripts: b.externalScripts[api.SaveArtifacts],
		ScriptsURL:      request.ScriptsURL,
		Location:        request.Location,
		Command:         api.SaveArtifacts,
		Stdout:          writer,
		OnStart:         extractFunc,
	}
	err = b.docker.RunContainer(opts)
	writer.Close()

	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewSaveArtifactsError(image, e.Output, err)
	}
	return err
}

// Execute runs the specified STI script in the builder image.
func (b *STI) Execute(command string, request *api.Request) error {
	glog.V(2).Infof("Using image name %s", request.BaseImage)

	uploadDir := filepath.Join(request.WorkingDir, "upload")
	tarFileName, err := b.tar.CreateTarFile(request.WorkingDir, uploadDir)
	if err != nil {
		return err
	}

	tarFile, err := b.fs.Open(tarFileName)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	errOutput := ""
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	defer errReader.Close()
	defer errWriter.Close()
	externalScripts := b.externalScripts[command]
	// if LayeredBuild is called then all the scripts will be placed inside the image
	if request.LayeredBuild {
		externalScripts = false
	}
	opts := docker.RunContainerOptions{
		Image:           request.BaseImage,
		Stdout:          outWriter,
		Stderr:          errWriter,
		PullImage:       request.ForcePull,
		ExternalScripts: externalScripts,
		ScriptsURL:      request.ScriptsURL,
		Location:        request.Location,
		Command:         command,
		Env:             b.generateConfigEnv(),
		PostExec:        b.postExecutor,
	}
	if !request.LayeredBuild {
		opts.Stdin = tarFile
	}
	// goroutine to stream container's output
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			if glog.V(2) || command == api.Usage {
				glog.Info(scanner.Text())
			}
		}
	}(outReader)
	// goroutine to stream container's error
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			text := scanner.Text()
			if glog.V(1) {
				glog.Errorf(text)
			}
			if len(errOutput) < maxErrorOutput {
				errOutput += text + "\n"
			}
		}
	}(errReader)

	err = b.docker.RunContainer(opts)
	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewContainerError(request.BaseImage, e.ErrorCode, errOutput)
	}
	return err
}

func (b *STI) generateConfigEnv() (configEnv []string) {
	if len(b.request.Environment) > 0 {
		for key, val := range b.request.Environment {
			configEnv = append(configEnv, key+"="+val)
		}
	}
	return
}

func isMissingRequirements(text string) bool {
	tar, _ := regexp.MatchString(`.*tar.*not found`, text)
	sh, _ := regexp.MatchString(`.*/bin/sh.*no such file or directory`, text)
	return tar || sh
}
