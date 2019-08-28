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

const org = "bblfsh"

// Image is a struct that eases work with docker image and tag handling
// represented in form org/name:tag
type Image struct {
	Org  string
	Name string
	Tag  string
}

// InstallDriver downloads driver's repository, checkouts to a given commit,
// builds docker image and installs driver's image to bblfshd container.
// Requires bblfshd container running.
func InstallDriver(language, commit string) error {
	image, err := DownloadAndBuildDriver(language, commit)
	if err != nil {
		return err
	}

	return installDriverToBblfshd(bblfshdContainer, language, image)
}

// DownloadAndBuildDriver creates directory in the temporary folder, clones driver's repository there, checkouts to a given commit
// and runs docker image build script.
// Arguments:
// language - name of the supported language(check docker/conf/drivers.json)
// commit - commit hash to checkout to
func DownloadAndBuildDriver(language, commit string) (*Image, error) {
	driver, err := getDriver(language)
	if err != nil {
		return nil, err
	}
	image := newImage(driver, commit)

	tmpDir, err := ioutil.TempDir("", driver)
	if err != nil {
		return nil, err
	}
	log.Debugf("Created temp directory %v", tmpDir)
	defer func() { os.RemoveAll(tmpDir) }()

	repo := driverURL(driver)
	log.Debugf("performing git clone repository %v %v", repo, commit)
	cloneCommand := fmt.Sprintf("git clone %[1]s %[2]s ; cd %[2]s ; git checkout %[3]s",
		shell.Quote(repo), shell.Quote(tmpDir), shell.Quote(commit))
	if err := performance.ExecCmd(cloneCommand); err != nil {
		return nil, err
	}

	log.Debugf("building with image %v", image)
	buildCommand := fmt.Sprintf("cd %s ; go run build.go %s", shell.Quote(tmpDir), shell.Quote(image.toString(true)))

	return image, performance.ExecCmd(buildCommand)
}

func getDriver(language string) (string, error) {
	data, err := staticfile.ReadAll("conf/drivers.json")
	if err != nil {
		return "", errors.NewKind("failed to read drivers config").Wrap(err)
	}

	drivers := make(map[string]string)
	if err := json.Unmarshal(data, &drivers); err != nil {
		return "", err
	}
	log.Debugf("Supported drivers: %v", drivers)

	driver, ok := drivers[language]
	if !ok {
		return "", errors.NewKind("language %v is not supported").New(language)
	}

	log.Debugf("Selected driver: %v", driver)
	return driver, nil
}

// Example of command: docker exec bblfshd-perf bblfshctl driver install python docker-daemon:bblfsh/python-driver:tag
func installDriverToBblfshd(bblfshContainer, language string, image *Image) error {
	installCommand := fmt.Sprintf("docker exec %s bblfshctl driver install %s docker-daemon:%s",
		shell.Quote(bblfshContainer), shell.Quote(language), shell.Quote(image.toString(true)))

	return performance.ExecCmd(installCommand)
}

func driverURL(driver string) string {
	return "https://github.com/bblfsh/" + driver
}

func newImage(name, tag string) *Image {
	return &Image{Org: org, Name: name, Tag: tag}
}

func (i *Image) toString(withTag bool) string {
	result := i.Org + "/" + i.Name
	if withTag {
		result += ":" + i.Tag
	}
	return result
}
