// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerfile/parser"
	dockerapi "github.com/docker/docker/client"
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build/strategies"
	"github.com/openshift/source-to-image/pkg/docker"
	dockerpkg "github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
	"github.com/openshift/source-to-image/pkg/util/fs"
	"golang.org/x/net/context"
)

const (
	DefaultDockerSocket = "unix:///var/run/docker.sock"
	TestSource          = "https://github.com/openshift/ruby-hello-world"

	FakeBuilderImage                = "sti_test/sti-fake"
	FakeUserImage                   = "sti_test/sti-fake-user"
	FakeImageScripts                = "sti_test/sti-fake-scripts"
	FakeImageScriptsNoSaveArtifacts = "sti_test/sti-fake-scripts-no-save-artifacts"
	FakeImageNoTar                  = "sti_test/sti-fake-no-tar"
	FakeImageOnBuild                = "sti_test/sti-fake-onbuild"
	FakeNumericUserImage            = "sti_test/sti-fake-numericuser"
	FakeImageOnBuildRootUser        = "sti_test/sti-fake-onbuild-rootuser"
	FakeImageOnBuildNumericUser     = "sti_test/sti-fake-onbuild-numericuser"
	FakeImageAssembleRoot           = "sti_test/sti-fake-assemble-root"
	FakeImageAssembleUser           = "sti_test/sti-fake-assemble-user"

	TagCleanBuild                              = "test/sti-fake-app"
	TagCleanBuildUser                          = "test/sti-fake-app-user"
	TagIncrementalBuild                        = "test/sti-incremental-app"
	TagIncrementalBuildUser                    = "test/sti-incremental-app-user"
	TagCleanBuildScripts                       = "test/sti-fake-app-scripts"
	TagIncrementalBuildScripts                 = "test/sti-incremental-app-scripts"
	TagIncrementalBuildScriptsNoSaveArtifacts  = "test/sti-incremental-app-scripts-no-save-artifacts"
	TagCleanLayeredBuildNoTar                  = "test/sti-fake-no-tar"
	TagCleanBuildOnBuild                       = "test/sti-fake-app-onbuild"
	TagIncrementalBuildOnBuild                 = "test/sti-incremental-app-onbuild"
	TagCleanBuildOnBuildNoName                 = "test/sti-fake-app-onbuild-noname"
	TagCleanBuildNoName                        = "test/sti-fake-app-noname"
	TagCleanLayeredBuildNoTarNoName            = "test/sti-fake-no-tar-noname"
	TagCleanBuildAllowedUIDsNamedUser          = "test/sti-fake-alloweduids-nameduser"
	TagCleanBuildAllowedUIDsNumericUser        = "test/sti-fake-alloweduids-numericuser"
	TagCleanBuildAllowedUIDsOnBuildRoot        = "test/sti-fake-alloweduids-onbuildroot"
	TagCleanBuildAllowedUIDsOnBuildNumericUser = "test/sti-fake-alloweduids-onbuildnumeric"
	TagCleanBuildAllowedUIDsAssembleRoot       = "test/sti-fake-alloweduids-assembleroot"
	TagCleanBuildAllowedUIDsAssembleUser       = "test/sti-fake-alloweduids-assembleuser"

	// Need to serve the scripts from local host so any potential changes to the
	// scripts are made available for integration testing.
	//
	// Port 23456 must match the port used in the fake image Dockerfiles
	FakeScriptsHTTPURL = "http://127.0.0.1:23456/.s2i/bin"
)

var engineClient docker.Client

func init() {
	var err error
	engineClient, err = docker.NewEngineAPIClient(docker.GetDefaultDockerConfig())
	if err != nil {
		panic(err)
	}

	// get the full path to this .go file so we can construct the file url
	// using this file's dirname
	_, filename, _, _ := runtime.Caller(0)
	testImagesDir := filepath.Join(filepath.Dir(filename), "scripts")

	l, err := net.Listen("tcp", ":23456")
	if err != nil {
		panic(err)
	}

	hs := http.Server{Handler: http.FileServer(http.Dir(testImagesDir))}
	hs.SetKeepAlivesEnabled(false)
	go hs.Serve(l)
}

func getDefaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 20*time.Second)
}

// TestInjectionBuild tests the build where we inject files to assemble script.
func TestInjectionBuild(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-test-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	err = ioutil.WriteFile(filepath.Join(tempdir, "secret"), []byte("secret"), 0666)
	if err != nil {
		t.Errorf("Unable to write content to temporary injection file: %v", err)
	}

	integration(t).exerciseInjectionBuild(TagCleanBuild, FakeBuilderImage, []string{
		tempdir + ":/tmp",
		tempdir + ":",
		tempdir + ":test;" + tempdir + ":test2",
	}, true)
}

func TestInjectionBuildBadDestination(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-test-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	err = ioutil.WriteFile(filepath.Join(tempdir, "secret"), []byte("secret"), 0666)
	if err != nil {
		t.Errorf("Unable to write content to temporary injection file: %v", err)
	}

	integration(t).exerciseInjectionBuild(TagCleanBuild, FakeBuilderImage, []string{tempdir + ":/bad/dir"}, false)
}

type integrationTest struct {
	t             *testing.T
	setupComplete bool
}

func (i integrationTest) InspectImage(name string) (*dockertypes.ImageInspect, error) {
	ctx, cancel := getDefaultContext()
	defer cancel()
	resp, _, err := engineClient.ImageInspectWithRaw(ctx, name)
	if err != nil {
		if dockerapi.IsErrImageNotFound(err) {
			return nil, fmt.Errorf("no such image :%q", name)
		}
		return nil, err
	}
	return &resp, nil
}

var (
	FakeScriptsFileURL string
)

func getLogLevel() (level int) {
	for level = 5; level >= 0; level-- {
		if glog.V(glog.Level(level)) == true {
			break
		}
	}
	return
}

// setup sets up integration tests
func (i *integrationTest) setup() {
	if !i.setupComplete {
		// get the full path to this .go file so we can construct the file url
		// using this file's dirname
		_, filename, _, _ := runtime.Caller(0)
		testImagesDir := filepath.Join(filepath.Dir(filename), "scripts")
		FakeScriptsFileURL = "file://" + filepath.ToSlash(filepath.Join(testImagesDir, ".s2i", "bin"))

		for _, image := range []string{TagCleanBuild, TagCleanBuildUser, TagIncrementalBuild, TagIncrementalBuildUser} {
			ctx, cancel := getDefaultContext()
			engineClient.ImageRemove(ctx, image, dockertypes.ImageRemoveOptions{})
			cancel()
		}

		i.setupComplete = true
	}

	from := flag.CommandLine
	if vflag := from.Lookup("v"); vflag != nil {
		// the thing here is that we are looking for the bash -v passed into test-integration.sh (with no value),
		// but for glog (https://github.com/golang/glog/blob/master/glog.go), one specifies
		// the logging level with -v=# (i.e. -v=0 or -v=3 or -v=5).
		// so, for the changes stemming from issue 133, we 'reuse' the bash -v, and set the highest glog level.
		// (if you look at STI's main.go, and setupGlog, it essentially maps glog's -v to --loglevel for use by the sti command)
		//NOTE - passing --loglevel or -v=5 into test-integration.sh does not work
		if getLogLevel() != 5 {
			vflag.Value.Set("5")
			// FIXME currently glog has only option to redirect output to stderr
			// the preferred for STI would be to redirect to stdout
			flag.CommandLine.Set("logtostderr", "true")
		}
	}
}

func integration(t *testing.T) *integrationTest {
	i := &integrationTest{t: t}
	i.setup()
	return i
}

// Test a clean build.  The simplest case.
func TestCleanBuild(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuild, false, FakeBuilderImage, "", true, true, false)
}

// Test Labels
func TestCleanBuildLabel(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuild, false, FakeBuilderImage, "", true, true, true)
}

func TestCleanBuildUser(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuildUser, false, FakeUserImage, "", true, true, false)
}

func TestCleanBuildFileScriptsURL(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuild, false, FakeBuilderImage, FakeScriptsFileURL, true, true, false)
}

func TestCleanBuildHttpScriptsURL(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuild, false, FakeBuilderImage, FakeScriptsHTTPURL, true, true, false)
}

func TestCleanBuildScripts(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuildScripts, false, FakeImageScripts, "", true, true, false)
}

func TestLayeredBuildNoTar(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanLayeredBuildNoTar, false, FakeImageNoTar, FakeScriptsFileURL, false, true, false)
}

// Test that a build config with a callbackURL will invoke HTTP endpoint
func TestCleanBuildCallbackInvoked(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuild, true, FakeBuilderImage, "", true, true, false)
}

func TestCleanBuildOnBuild(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuildOnBuild, false, FakeImageOnBuild, "", true, true, false)
}

func TestCleanBuildOnBuildNoName(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuildOnBuildNoName, false, FakeImageOnBuild, "", false, false, false)
}

func TestCleanBuildNoName(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanBuildNoName, false, FakeBuilderImage, "", true, false, false)
}

func TestLayeredBuildNoTarNoName(t *testing.T) {
	integration(t).exerciseCleanBuild(TagCleanLayeredBuildNoTarNoName, false, FakeImageNoTar, FakeScriptsFileURL, false, false, false)
}

func TestAllowedUIDsNamedUser(t *testing.T) {
	integration(t).exerciseCleanAllowedUIDsBuild(TagCleanBuildAllowedUIDsNamedUser, FakeUserImage, true)
}

func TestAllowedUIDsNumericUser(t *testing.T) {
	integration(t).exerciseCleanAllowedUIDsBuild(TagCleanBuildAllowedUIDsNumericUser, FakeNumericUserImage, false)
}

func TestAllowedUIDsOnBuildRootUser(t *testing.T) {
	integration(t).exerciseCleanAllowedUIDsBuild(TagCleanBuildAllowedUIDsNamedUser, FakeImageOnBuildRootUser, true)
}

func TestAllowedUIDsOnBuildNumericUser(t *testing.T) {
	integration(t).exerciseCleanAllowedUIDsBuild(TagCleanBuildAllowedUIDsNumericUser, FakeImageOnBuildNumericUser, false)
}

func TestAllowedUIDsAssembleRoot(t *testing.T) {
	integration(t).exerciseCleanAllowedUIDsBuild(TagCleanBuildAllowedUIDsAssembleRoot, FakeImageAssembleRoot, true)
}

func TestAllowedUIDsAssembleUser(t *testing.T) {
	integration(t).exerciseCleanAllowedUIDsBuild(TagCleanBuildAllowedUIDsAssembleUser, FakeImageAssembleUser, false)
}

func (i *integrationTest) exerciseCleanAllowedUIDsBuild(tag, imageName string, expectError bool) {
	t := i.t
	config := &api.Config{
		DockerConfig:      docker.GetDefaultDockerConfig(),
		BuilderImage:      imageName,
		BuilderPullPolicy: api.DefaultBuilderPullPolicy,
		Source:            git.MustParse(TestSource),
		Tag:               tag,
		Incremental:       false,
		ScriptsURL:        "",
		ExcludeRegExp:     tar.DefaultExclusionPattern.String(),
	}
	config.AllowedUIDs.Set("1-")
	_, _, err := strategies.GetStrategy(engineClient, config)
	if err != nil && !expectError {
		t.Fatalf("Cannot create a new builder: %v", err)
	}
	if err == nil && expectError {
		t.Fatalf("Did not get an error and was expecting one.")
	}
}

func (i *integrationTest) exerciseCleanBuild(tag string, verifyCallback bool, imageName string, scriptsURL string, expectImageName bool, setTag bool, checkLabel bool) {
	t := i.t
	callbackURL := ""
	callbackInvoked := false
	callbackHasValidJSON := false
	if verifyCallback {
		handler := func(w http.ResponseWriter, r *http.Request) {
			// we got called
			callbackInvoked = true
			// the header is as expected
			contentType := r.Header["Content-Type"][0]
			callbackHasValidJSON = contentType == "application/json"
			// the request body is as expected
			if callbackHasValidJSON {
				defer r.Body.Close()
				body, _ := ioutil.ReadAll(r.Body)
				type CallbackMessage struct {
					Success bool
					Labels  map[string]string
				}
				var callbackMessage CallbackMessage
				err := json.Unmarshal(body, &callbackMessage)
				callbackHasValidJSON = (err == nil) && callbackMessage.Success && len(callbackMessage.Labels) > 0
			}
		}
		ts := httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()
		callbackURL = ts.URL
	}

	var buildTag string
	if setTag {
		buildTag = tag
	} else {
		buildTag = ""
	}

	config := &api.Config{
		DockerConfig:      docker.GetDefaultDockerConfig(),
		BuilderImage:      imageName,
		BuilderPullPolicy: api.DefaultBuilderPullPolicy,
		Source:            git.MustParse(TestSource),
		Tag:               buildTag,
		Incremental:       false,
		CallbackURL:       callbackURL,
		ScriptsURL:        scriptsURL,
		ExcludeRegExp:     tar.DefaultExclusionPattern.String(),
	}

	b, _, err := strategies.GetStrategy(engineClient, config)
	if err != nil {
		t.Fatalf("Cannot create a new builder.")
	}
	resp, err := b.Build(config)
	if err != nil {
		t.Fatalf("An error occurred during the build: %v", err)
	} else if !resp.Success {
		t.Fatalf("The build failed.")
	}
	if callbackInvoked != verifyCallback {
		t.Fatalf("S2I build did not invoke callback")
	}
	if callbackHasValidJSON != verifyCallback {
		t.Fatalf("S2I build did not invoke callback with valid json message")
	}

	// We restrict this check to only when we are passing tag through the build config
	// since we will not end up with an available tag by that name from build
	if setTag {
		i.checkForImage(tag)
		containerID := i.createContainer(tag)
		i.checkBasicBuildState(containerID, resp.WorkingDir)

		if checkLabel {
			i.checkForLabel(tag)
		}

		i.removeContainer(containerID)
	}

	// Check if we receive back an ImageID when we are expecting to
	if expectImageName && len(resp.ImageID) == 0 {
		t.Fatalf("S2I build did not receive an ImageID in response")
	}
	if !expectImageName && len(resp.ImageID) > 0 {
		t.Fatalf("S2I build received an ImageID in response")
	}
}

// Test an incremental build.
func TestIncrementalBuildAndRemovePreviousImage(t *testing.T) {
	integration(t).exerciseIncrementalBuild(TagIncrementalBuild, FakeBuilderImage, true, false, false)
}

func TestIncrementalBuildAndKeepPreviousImage(t *testing.T) {
	integration(t).exerciseIncrementalBuild(TagIncrementalBuild, FakeBuilderImage, false, false, false)
}

func TestIncrementalBuildUser(t *testing.T) {
	integration(t).exerciseIncrementalBuild(TagIncrementalBuildUser, FakeBuilderImage, true, false, false)
}

func TestIncrementalBuildScripts(t *testing.T) {
	integration(t).exerciseIncrementalBuild(TagIncrementalBuildScripts, FakeImageScripts, true, false, false)
}

func TestIncrementalBuildScriptsNoSaveArtifacts(t *testing.T) {
	integration(t).exerciseIncrementalBuild(TagIncrementalBuildScriptsNoSaveArtifacts, FakeImageScriptsNoSaveArtifacts, true, true, false)
}

func TestIncrementalBuildOnBuild(t *testing.T) {
	integration(t).exerciseIncrementalBuild(TagIncrementalBuildOnBuild, FakeImageOnBuild, false, true, true)
}

func (i *integrationTest) exerciseInjectionBuild(tag, imageName string, injections []string, expectSuccess bool) {
	t := i.t

	injectionList := api.VolumeList{}
	for _, i := range injections {
		err := injectionList.Set(i)
		if err != nil {
			t.Errorf("injectionList.Set() failed with error %s\n", err)
		}
	}
	// For test purposes, keep at least one injected source
	var keptVolume *api.VolumeSpec
	if len(injectionList) > 0 {
		injectionList[0].Keep = true
		keptVolume = &injectionList[0]
	}
	config := &api.Config{
		DockerConfig:      docker.GetDefaultDockerConfig(),
		BuilderImage:      imageName,
		BuilderPullPolicy: api.DefaultBuilderPullPolicy,
		Source:            git.MustParse(TestSource),
		Tag:               tag,
		Injections:        injectionList,
		ExcludeRegExp:     tar.DefaultExclusionPattern.String(),
	}
	builder, _, err := strategies.GetStrategy(engineClient, config)
	if err != nil {
		t.Fatalf("Unable to create builder: %v", err)
	}
	resp, err := builder.Build(config)
	if !expectSuccess {
		if resp.Success {
			t.Fatal("Success was returned, but should have failed")
		}
		return
	}
	if err != nil {
		t.Fatalf("Unexpected error occurred during build: %v", err)
	}
	if !resp.Success {
		t.Fatalf("S2I build failed.")
	}
	i.checkForImage(tag)
	containerID := i.createContainer(tag)
	defer i.removeContainer(containerID)

	// Check that the injected file is delivered to assemble script
	i.fileExists(containerID, "/sti-fake/secret-delivered")
	i.fileExists(containerID, "/sti-fake/relative-secret-delivered")

	// Make sure the injected file does not exists in resulting image
	testFs := fs.NewFileSystem()
	files, err := util.ListFilesToTruncate(testFs, injectionList)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	for _, f := range files {
		if err = i.testFile(tag, f); err == nil {
			t.Errorf("The file %q must be empty or not exist", f)
		}
	}
	if keptVolume != nil {
		keptFiles, err := util.ListFiles(testFs, *keptVolume)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		for _, f := range keptFiles {
			if err = i.testFile(tag, f); err != nil {
				t.Errorf("The file %q must exist and not be empty", f)
			}
		}
	}
}

func (i *integrationTest) testFile(tag, path string) error {
	exitCode := i.runInImage(tag, "test -s "+path)
	if exitCode != 0 {
		return fmt.Errorf("file %s does not exist or is empty in the container %s", path, tag)
	}
	return nil
}

func (i *integrationTest) exerciseIncrementalBuild(tag, imageName string, removePreviousImage bool, expectClean bool, checkOnBuild bool) {
	t := i.t
	start := time.Now()
	config := &api.Config{
		DockerConfig:        docker.GetDefaultDockerConfig(),
		BuilderImage:        imageName,
		BuilderPullPolicy:   api.DefaultBuilderPullPolicy,
		Source:              git.MustParse(TestSource),
		Tag:                 tag,
		Incremental:         false,
		RemovePreviousImage: removePreviousImage,
		ExcludeRegExp:       tar.DefaultExclusionPattern.String(),
	}

	builder, _, err := strategies.GetStrategy(engineClient, config)
	if err != nil {
		t.Fatalf("Unable to create builder: %v", err)
	}
	resp, err := builder.Build(config)
	if err != nil {
		t.Fatalf("Unexpected error occurred during build: %v", err)
	}
	if !resp.Success {
		t.Fatalf("S2I build failed.")
	}

	previousImageID := resp.ImageID
	config = &api.Config{
		DockerConfig:            docker.GetDefaultDockerConfig(),
		BuilderImage:            imageName,
		BuilderPullPolicy:       api.DefaultBuilderPullPolicy,
		Source:                  git.MustParse(TestSource),
		Tag:                     tag,
		Incremental:             true,
		RemovePreviousImage:     removePreviousImage,
		PreviousImagePullPolicy: api.PullIfNotPresent,
		ExcludeRegExp:           tar.DefaultExclusionPattern.String(),
	}

	builder, _, err = strategies.GetStrategy(engineClient, config)
	if err != nil {
		t.Fatalf("Unable to create incremental builder: %v", err)
	}
	resp, err = builder.Build(config)
	if err != nil {
		t.Fatalf("Unexpected error occurred during incremental build: %v", err)
	}
	if !resp.Success {
		t.Fatalf("S2I incremental build failed.")
	}

	i.checkForImage(tag)
	containerID := i.createContainer(tag)
	defer i.removeContainer(containerID)
	i.checkIncrementalBuildState(containerID, resp.WorkingDir, expectClean)

	_, err = i.InspectImage(previousImageID)
	if removePreviousImage {
		if err == nil {
			t.Errorf("Previous image %s not deleted", previousImageID)
		}
	} else {
		if err != nil {
			t.Errorf("Couldn't find previous image %s", previousImageID)
		}
	}

	if checkOnBuild {
		i.fileExists(containerID, "/sti-fake/src/onbuild")
	}

	if took := time.Since(start); took > docker.DefaultDockerTimeout {
		// https://github.com/openshift/source-to-image/issues/301 is a
		// case where incremental builds would get stuck until the
		// timeout.
		t.Errorf("Test took too long (%v), some operation may have gotten stuck waiting for the DefaultDockerTimeout (%v). Inspect the logs to find operations that took long.", took, docker.DefaultDockerTimeout)
	}
}

func TestDockerfileBuild(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: tempdir + string(os.PathSeparator) + "MyDockerfile",
	}
	expected := []string{
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"\"io.openshift.s2i.build.commit.date\"",
		"\"io.openshift.s2i.build.commit.id\"",
		"\"io.openshift.s2i.build.commit.ref\"",
		"\"io.openshift.s2i.build.commit.message\"",
		"\"io.openshift.s2i.build.source-location\"",
		"\"io.openshift.s2i.build.image\"=\"docker.io/centos/nodejs-8-centos7\"",
		"\"io.openshift.s2i.build.commit.author\"",
		"(?m)^COPY upload/src /tmp/src",
		"(?m)^RUN chown -R 1001:0.* /tmp/src",
		// Ensure we are using the default image user when running assemble
		"(?m)^USER 1001\n.+\n.+\nRUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "MyDockerfile"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildDefaultDockerfile(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: tempdir + string(os.PathSeparator),
	}
	expected := []string{
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"\"io.openshift.s2i.build.commit.date\"",
		"\"io.openshift.s2i.build.commit.id\"",
		"\"io.openshift.s2i.build.commit.ref\"",
		"\"io.openshift.s2i.build.commit.message\"",
		"\"io.openshift.s2i.build.source-location\"",
		"\"io.openshift.s2i.build.image\"=\"docker.io/centos/nodejs-8-centos7\"",
		"\"io.openshift.s2i.build.commit.author\"",
		"(?m)^COPY upload/src /tmp/src",
		"(?m)^RUN chown -R 1001:0.* /tmp/src",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "Dockerfile"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildEnv(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{
			{
				Name:  "key1",
				Value: "value1",
			},
			{
				Name:  "key2",
				Value: "value2",
			},
		},
		Labels: map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"key1=\"value1\"",
		"key2=\"value2\"",
	}
	runDockerfileTest(t, config, expected, nil, nil)
}

func TestDockerfileBuildLabels(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels: map[string]string{"label1": "value1",
			"label2": "value2",
			"io.openshift.s2i.build.commit.author": "shadowman"},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"\"io.openshift.s2i.build.commit.date\"",
		"\"io.openshift.s2i.build.commit.id\"",
		"\"io.openshift.s2i.build.commit.ref\"",
		"\"io.openshift.s2i.build.commit.message\"",
		"\"io.openshift.s2i.build.source-location\"",
		"\"io.openshift.s2i.build.image\"=\"docker.io/centos/nodejs-8-centos7\"",
		"\"io.openshift.s2i.build.commit.author\"=\"shadowman\"",
		"\"label1\"=\"value1\"",
		"\"label2\"=\"value2\"",
	}
	runDockerfileTest(t, config, expected, nil, nil)
}

func TestDockerfileBuildInjections(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	injection1 := filepath.Join(tempdir, "injection1")
	err = os.Mkdir(injection1, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err = ioutil.TempFile(injection1, "injectfile-")
		if err != nil {
			t.Errorf("Unable to create injection file: %v", err)
		}
	}

	injection2 := filepath.Join(tempdir, "injection2")
	err = os.Mkdir(injection2, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}
	_, err = ioutil.TempFile(injection2, "injectfile-2")
	if err != nil {
		t.Errorf("Unable to create injection file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "/workdir",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections: api.VolumeList{
			{
				Source:      injection1,
				Destination: "injection1",
				Keep:        false,
			},
			{
				Source:      injection2,
				Destination: "/destination/injection2",
				Keep:        true,
			},
		},
		Destination: "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	// strip the C: from windows paths because it's not valid in the middle of a path
	// like upload/injections/C:/tempdir/injection1
	trimmedInjection1 := filepath.ToSlash(strings.TrimPrefix(injection1, filepath.VolumeName(injection1)))
	trimmedInjection2 := filepath.ToSlash(strings.TrimPrefix(injection2, filepath.VolumeName(injection2)))

	expected := []string{
		"(?m)^COPY upload/injections" + trimmedInjection1 + " /workdir/injection1",
		"(?m)^RUN chown -R 1001:0.* /workdir/injection1",
		"(?m)^COPY upload/injections" + trimmedInjection2 + " /destination/injection2",
		"(?m)^RUN chown -R 1001:0.* /destination/injection2",
		"(?m)^RUN rm /workdir/injection1/injectfile-",
		"    rm /workdir/injection1/injectfile-",
	}
	notExpected := []string{
		"rm -rf /destination/injection2",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/injections"+trimmedInjection1),
		filepath.Join(tempdir, "upload/injections"+trimmedInjection2),
	}
	runDockerfileTest(t, config, expected, notExpected, expectedFiles)
}

func TestDockerfileBuildScriptsURLAssemble(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	assemble := filepath.Join(tempdir, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildScriptsURLRun(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	run := filepath.Join(tempdir, "run")
	_, err = os.OpenFile(run, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create run file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /destination/scripts/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/run"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildSourceScriptsAssemble(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	assemble := filepath.Join(sourcescripts, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:    true,
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildSourceScriptsRun(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	run := filepath.Join(sourcescripts, "run")
	_, err = os.OpenFile(run, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create run file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:    true,
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /destination/scripts/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/run"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

// TestDockerfileBuildScriptsURLImage tests the behavior if the ScriptsURL
// is set to an image:// URL. In this case we blind trust that the image
// contains all of the s2i scripts at the given directory, regardless
// of what is contained in the source.
func TestDockerfileBuildScriptsURLImage(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	assemble := filepath.Join(sourcescripts, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:    true,
		ScriptsURL:   "image:///usr/custom/s2i",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^RUN /usr/custom/s2i/assemble",
		"(?m)^CMD /usr/custom/s2i/run",
	}
	notExpected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
	}
	runDockerfileTest(t, config, expected, notExpected, nil)
}

func TestDockerfileBuildImageScriptsURLAssemble(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	assemble := filepath.Join(tempdir, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ImageScriptsURL: "file://" + filepath.ToSlash(tempdir),
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildImageScriptsURLRun(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	run := filepath.Join(tempdir, "run")
	_, err = os.OpenFile(run, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create run file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ImageScriptsURL: "file://" + filepath.ToSlash(tempdir),
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /destination/scripts/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/run"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildImageScriptsURLImage(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	assemble := filepath.Join(sourcescripts, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:       true,
		ImageScriptsURL: "image:///usr/custom/s2i",
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/custom/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileBuildScriptsAndImageURL(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	assemble := filepath.Join(tempdir, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:      "file://" + filepath.ToSlash(tempdir),
		ImageScriptsURL: "image:///usr/some/dir",
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/some/dir/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

// TestDockerfileBuildScriptsAndImageURLConflicts tests if both
// the ScriptsURL and ImageScriptsURL point to a non-image directory.
// In this event, the ScriptsURL value should take precedence.
func TestDockerfileBuildScriptsAndImageURLConflicts(t *testing.T) {
	scriptsTempDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(scriptsTempDir)

	imageTempDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(imageTempDir)

	outputDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(outputDir)

	scriptsAssemble := filepath.Join(scriptsTempDir, "assemble")
	assembleData := []byte("#!/bin/bash\necho \"Hello World!\"")
	err = ioutil.WriteFile(scriptsAssemble, assembleData, 0666)
	if err != nil {
		t.Errorf("Unable to create image assemble file: %v", err)
	}

	imageAssemble := filepath.Join(imageTempDir, "assemble")
	_, err = os.OpenFile(imageAssemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:      "file://" + filepath.ToSlash(scriptsTempDir),
		ImageScriptsURL: "file://" + filepath.ToSlash(imageTempDir),
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(outputDir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(outputDir, "upload/src/server.js"),
		filepath.Join(outputDir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles)
	dockerfileAssemble, err := ioutil.ReadFile(filepath.Join(outputDir, "upload/scripts/assemble"))
	if err != nil {
		t.Errorf("Failed to read uploaded assemble script: %v", err)
	}
	if string(dockerfileAssemble) != string(assembleData) {
		t.Errorf("Expected uploaded assemble script:\n\n%s\n\nto be:\n\n%s", dockerfileAssemble, assembleData)
	}
}

func TestDockerfileIncrementalBuild(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Tag:          "test:tag",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER 1001",
		"(?m)^RUN if \\[ -s /usr/libexec/s2i/save-artifacts \\]; then /usr/libexec/s2i/save-artifacts > /tmp/artifacts.tar; else touch /tmp/artifacts.tar; fi",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"(?m)^COPY --from=cached /tmp/artifacts.tar /tmp/artifacts.tar",
		"(?m)^RUN chown -R 1001:0.* /tmp/artifacts.tar",
		"if \\[ -s /tmp/artifacts.tar \\]; then mkdir -p /tmp/artifacts; tar -xf /tmp/artifacts.tar -C /tmp/artifacts; fi",
		"rm /tmp/artifacts.tar",
		"(?m)^COPY upload/src /tmp/src",
		"(?m)^RUN chown -R 1001:0.* /tmp/src",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}

	runDockerfileTest(t, config, expected, nil, nil)
}

func TestDockerfileIncrementalSourceSave(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	saveArtifacts := filepath.Join(sourcescripts, "save-artifacts")
	_, err = os.OpenFile(saveArtifacts, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create save-artifacts file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ScriptsURL:   "",
		Tag:          "test:tag",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER root\n",
		"(?m)^COPY upload/scripts/save-artifacts /destination/scripts/save-artifacts",
		"(?m)^RUN chown .*1001:0 /destination/scripts/save-artifacts",
		"(?m)^USER 1001\nRUN if \\[ -s /destination/scripts/save-artifacts \\]; then /destination/scripts/save-artifacts > /tmp/artifacts.tar;",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"mkdir -p /destination/artifacts",
		"tar -xf /tmp/artifacts.tar -C /destination/artifacts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/save-artifacts"),
	}

	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileIncrementalSaveURL(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	saveArtifacts := filepath.Join(tempdir, "save-artifacts")
	_, err = os.OpenFile(saveArtifacts, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create save-artifacts file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Tag:          "test:tag",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER root\n",
		"(?m)^COPY upload/scripts/save-artifacts /destination/scripts/save-artifacts",
		"(?m)^RUN chown 1001:0 /destination/scripts/save-artifacts",
		"(?m)^USER 1001\nRUN if \\[ -s /destination/scripts/save-artifacts \\]; then /destination/scripts/save-artifacts > /tmp/artifacts.tar;",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"mkdir -p /destination/artifacts",
		"tar -xf /tmp/artifacts.tar -C /destination/artifacts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/save-artifacts"),
	}

	runDockerfileTest(t, config, expected, nil, expectedFiles)
}

func TestDockerfileIncrementalTag(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage:       "docker.io/centos/nodejs-8-centos7",
		AssembleUser:       "",
		ImageWorkDir:       "",
		Incremental:        true,
		Source:             git.MustParse("https://github.com/sclorg/nodejs-ex"),
		Tag:                "test:tag",
		IncrementalFromTag: "incremental:tag",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM incremental:tag as cached",
		"/usr/libexec/s2i/save-artifacts > /tmp/artifacts.tar",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"mkdir -p /tmp/artifacts",
		"tar -xf /tmp/artifacts.tar -C /tmp/artifacts",
		"rm /tmp/artifacts.tar",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}

	runDockerfileTest(t, config, expected, nil, nil)
}

func TestDockerfileIncrementalAssembleUser(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "2250",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		Tag:          "test:tag",
		Environment:  api.EnvironmentList{},
		Labels:       map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER 2250",
		"/usr/libexec/s2i/save-artifacts > /tmp/artifacts.tar",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"(?m)^COPY --from=cached /tmp/artifacts.tar /tmp/artifacts.tar",
		"(?m)^RUN chown -R 2250:0 .*/tmp/artifacts.tar",
		"mkdir -p /tmp/artifacts",
		"tar -xf /tmp/artifacts.tar -C /tmp/artifacts",
		"rm /tmp/artifacts.tar",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}

	runDockerfileTest(t, config, expected, nil, nil)
}

func runDockerfileTest(t *testing.T, config *api.Config, expected []string, notExpected []string, expectedFiles []string) {

	b, _, err := strategies.GetStrategy(nil, config)
	if err != nil {
		t.Fatalf("Cannot create a new builder.")
	}
	resp, err := b.Build(config)
	if err != nil {
		t.Fatalf("An error occurred during the build: %v", err)
	} else if !resp.Success {
		t.Fatalf("The build failed.")
	}

	filebytes, err := ioutil.ReadFile(config.AsDockerfile)
	if err != nil {
		t.Fatalf("An error occurred reading the dockerfile: %v", err)
	}
	dockerfile := string(filebytes)

	buf := bytes.NewBuffer(filebytes)
	_, err = parser.Parse(buf)
	if err != nil {
		t.Fatalf("An error occurred parsing the dockerfile: %v\n%s", err, dockerfile)
	}

	for _, s := range expected {
		reg, err := regexp.Compile(s)
		if err != nil {
			t.Fatalf("failed to compile regex %q: %v", s, err)
		}
		if !reg.MatchString(dockerfile) {
			t.Fatalf("Expected dockerfile to contain %s, it did not: \n%s", s, dockerfile)
		}
	}
	for _, s := range notExpected {
		reg, err := regexp.Compile(s)
		if err != nil {
			t.Fatalf("failed to compile regex %q: %v", s, err)
		}
		if reg.MatchString(dockerfile) {
			t.Fatalf("Expected dockerfile not to contain %s, it did: \n%s", s, dockerfile)
		}
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("Did not find expected file %s, ", f)
		}
	}
}

// Support methods
func (i *integrationTest) checkForImage(tag string) {
	_, err := i.InspectImage(tag)
	if err != nil {
		i.t.Errorf("Couldn't find image with tag: %s", tag)
	}
}

func (i *integrationTest) createContainer(image string) string {
	ctx, cancel := getDefaultContext()
	defer cancel()
	opts := dockertypes.ContainerCreateConfig{Name: "", Config: &dockercontainer.Config{Image: image}}
	container, err := engineClient.ContainerCreate(ctx, opts.Config, opts.HostConfig, opts.NetworkingConfig, opts.Name)
	if err != nil {
		i.t.Errorf("Couldn't create container from image %s with error %+v", image, err)
		return ""
	}

	ctx, cancel = getDefaultContext()
	defer cancel()
	err = engineClient.ContainerStart(ctx, container.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		i.t.Errorf("Couldn't start container: %s with error %+v", container.ID, err)
		return ""
	}

	ctx, cancel = getDefaultContext()
	defer cancel()
	waitC, errC := engineClient.ContainerWait(ctx, container.ID, dockercontainer.WaitConditionNextExit)
	select {
	case result := <-waitC:
		if result.StatusCode != 0 {
			i.t.Errorf("Bad exit code from container: %d", result.StatusCode)
			return ""
		}
	case err := <-errC:
		i.t.Errorf("Error waiting for container: %v", err)
		return ""
	}
	return container.ID
}

func (i *integrationTest) runInContainer(image string, command []string) int {
	ctx, cancel := getDefaultContext()
	defer cancel()
	opts := dockertypes.ContainerCreateConfig{Name: "", Config: &dockercontainer.Config{Image: image, AttachStdout: false, AttachStdin: false, Cmd: command}}
	container, err := engineClient.ContainerCreate(ctx, opts.Config, opts.HostConfig, opts.NetworkingConfig, opts.Name)
	if err != nil {
		i.t.Errorf("Couldn't create container from image %s err %+v", image, err)
		return -1
	}

	ctx, cancel = getDefaultContext()
	defer cancel()
	err = engineClient.ContainerStart(ctx, container.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		i.t.Errorf("Couldn't start container: %s", container.ID)
	}
	ctx, cancel = getDefaultContext()
	defer cancel()
	waitC, errC := engineClient.ContainerWait(ctx, container.ID, dockercontainer.WaitConditionNextExit)
	exitCode := -1
	select {
	case result := <-waitC:
		exitCode = int(result.StatusCode)
	case err := <-errC:
		i.t.Errorf("Couldn't wait for container: %s: %v", container.ID, err)
	}
	ctx, cancel = getDefaultContext()
	defer cancel()
	err = engineClient.ContainerRemove(ctx, container.ID, dockertypes.ContainerRemoveOptions{})
	if err != nil {
		i.t.Errorf("Couldn't remove container: %s", container.ID)
	}
	return exitCode
}

func (i *integrationTest) removeContainer(cID string) {
	ctx, cancel := getDefaultContext()
	defer cancel()
	engineClient.ContainerKill(ctx, cID, "SIGKILL")
	removeOpts := dockertypes.ContainerRemoveOptions{
		RemoveVolumes: true,
	}
	err := engineClient.ContainerRemove(ctx, cID, removeOpts)
	if err != nil {
		i.t.Errorf("Couldn't remove container %s: %s", cID, err)
	}
}

func (i *integrationTest) fileExists(cID string, filePath string) {
	res := i.fileExistsInContainer(cID, filePath)

	if !res {
		i.t.Errorf("Couldn't find file %s in container %s", filePath, cID)
	}
}

func (i *integrationTest) fileNotExists(cID string, filePath string) {
	res := i.fileExistsInContainer(cID, filePath)

	if res {
		i.t.Errorf("Unexpected file %s in container %s", filePath, cID)
	}
}

func (i *integrationTest) runInImage(image string, cmd string) int {
	return i.runInContainer(image, []string{"/bin/sh", "-c", cmd})
}

func (i *integrationTest) checkBasicBuildState(cID string, workingDir string) {
	i.fileExists(cID, "/sti-fake/assemble-invoked")
	i.fileExists(cID, "/sti-fake/run-invoked")
	i.fileExists(cID, "/sti-fake/src/Gemfile")

	_, err := os.Stat(workingDir)
	if !os.IsNotExist(err) {
		i.t.Errorf("Unexpected error from stat check on %s", workingDir)
	}
}

func (i *integrationTest) checkIncrementalBuildState(cID string, workingDir string, expectClean bool) {
	i.checkBasicBuildState(cID, workingDir)
	if expectClean {
		i.fileNotExists(cID, "/sti-fake/save-artifacts-invoked")
	} else {
		i.fileExists(cID, "/sti-fake/save-artifacts-invoked")
	}
}

func (i *integrationTest) fileExistsInContainer(cID string, filePath string) bool {
	ctx, cancel := getDefaultContext()
	defer cancel()
	rdr, stats, err := engineClient.CopyFromContainer(ctx, cID, filePath)
	if err != nil {
		return false
	}
	defer rdr.Close()
	return "" != stats.Name
}

func (i *integrationTest) checkForLabel(image string) {
	docker := dockerpkg.New(engineClient, (&api.Config{}).PullAuthentication)

	labelMap, err := docker.GetLabels(image)
	if err != nil {
		i.t.Fatalf("Unable to get labels from image %s: %v", image, err)
	}

	if labelMap["testLabel"] != "testLabel_value" {
		i.t.Errorf("Unable to verify 'testLabel' for image '%s'", image)
	}
}
