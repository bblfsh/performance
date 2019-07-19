package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/bblfsh/performance"

	"bitbucket.org/creachadair/shell"
	"github.com/creachadair/staticfile"
	"gopkg.in/src-d/go-errors.v1"
	"gopkg.in/src-d/go-log.v1"
)

//go:generate go run github.com/creachadair/staticfile/compiledata -pkg docker -out static.go conf/drivers.json

// InstallDriver downloads driver's repository, checkouts to a given commit,
// builds docker image and installs driver's image to bblfshd container.
// Requires bblfshd container running.
func InstallDriver(language, commit string) error {
	data, err := staticfile.ReadAll("conf/drivers.json")
	if err != nil {
		return errors.NewKind("failed to read drivers config").Wrap(err)
	}

	drivers := make(map[string]string)
	if err := json.Unmarshal(data, &drivers); err != nil {
		return err
	}
	log.Debugf("Supported drivers: %v", drivers)

	driver, ok := drivers[language]
	if !ok {
		return errors.NewKind("language %v is not supported").New(language)
	}
	log.Debugf("Selected driver: %v", driver)

	image := driverImage(driver, commit)
	if err := downloadAndBuildDriver(driver, commit, image); err != nil {
		return err
	}

	return installDriverToBblfshd(bblfshdContainer, language, image)
}

// downloadAndBuildDriver creates directory in the temporary folder, clones driver's repository there, checkouts to a given commit
// and runs docker image build script.
// Arguments:
// driver - name of the supported driver(check docker/conf/drivers.json)
// commit - commit hash to checkout to
// image - desired docker image name as the result of the build
func downloadAndBuildDriver(driver, commit, image string) error {
	tmpDir, err := ioutil.TempDir("", driver)
	if err != nil {
		return err
	}
	log.Debugf("Created temp directory %v", tmpDir)
	defer func() { os.RemoveAll(tmpDir) }()

	repo := driverURL(driver)
	log.Debugf("performing git clone repository %v %v", repo, commit)
	cloneCommand := fmt.Sprintf("git clone %[1]s %[2]s ; cd %[2]s ; git checkout %[3]s",
		shell.Quote(repo), shell.Quote(tmpDir), shell.Quote(commit))
	if err := performance.ExecCmd(cloneCommand); err != nil {
		return err
	}

	log.Debugf("building with image %v", image)
	buildCommand := fmt.Sprintf("cd %s ; go run build.go %s", shell.Quote(tmpDir), shell.Quote(image))

	return performance.ExecCmd(buildCommand)
}

// Example of command: docker exec bblfshd-perf bblfshctl driver install python docker-daemon:bblfsh/python-driver:tag
func installDriverToBblfshd(bblfshContainer, language, image string) error {
	installCommand := fmt.Sprintf("docker exec %s bblfshctl driver install %s docker-daemon:%s",
		shell.Quote(bblfshContainer), shell.Quote(language), shell.Quote(image))

	return performance.ExecCmd(installCommand)
}

func driverURL(driver string) string {
	return "https://github.com/bblfsh/" + driver
}

func driverImage(driver, tag string) string {
	return "bblfsh/" + driver + ":" + tag
}
