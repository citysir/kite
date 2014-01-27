package build

import (
	"errors"
	"fmt"
	"io/ioutil"
	"koding/kite/kd/util"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

type Build struct {
	appName    string
	version    string
	output     string
	binaryPath string
}

func NewBuild() *Build {
	return &Build{}
}

func (b *Build) Definition() string {
	return "Build deployable install packages"
}

func (b *Build) Exec(args []string) error {
	if len(args) == 0 {
		return errors.New("Usage: kd build <importPath>")
	}

	// use binary name as appName
	appName := filepath.Base(args[0])

	build := &Build{
		appName:    appName,
		version:    "0.0.1",
		binaryPath: args[0],
	}

	err := build.do()
	if err != nil {
		return err
	}

	fmt.Println("build successfull")
	return nil
}

func (b *Build) do() error {
	switch runtime.GOOS {
	case "darwin":
		return b.darwin()
	default:
		return fmt.Errorf("not supported os: %s.\n", runtime.GOOS)
	}
}

func (b *Build) linux() error {
	return nil
}

// darwin is building a new .pkg installer for darwin based OS'es.
func (b *Build) darwin() error {
	version := b.version
	if b.output == "" {
		b.output = fmt.Sprintf("koding-%s", b.appName)
	}

	scriptDir := "./darwin/scripts"
	installRoot := "./root" // TODO REMOVE

	os.RemoveAll(installRoot) // clean up old build before we continue
	installRootUsr := filepath.Join(installRoot, "/usr/local/bin")

	os.MkdirAll(installRootUsr, 0755)
	err := util.CopyFile(b.binaryPath, installRootUsr+"/"+b.appName)
	if err != nil {
		return err
	}

	tempDest, err := ioutil.TempDir("", "tempDest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDest)

	b.createScripts(scriptDir)
	b.createLaunchAgent(installRoot)

	cmdPkg := exec.Command("pkgbuild",
		"--identifier", fmt.Sprintf("com.koding.kite.%s.pkg", b.appName),
		"--version", version,
		"--scripts", scriptDir,
		"--root", installRoot,
		"--install-location", "/",
		fmt.Sprintf("%s/com.koding.kite.%s.pkg", tempDest, b.appName),
		// used for next step, also set up for distribution.xml
	)

	_, err = cmdPkg.CombinedOutput()
	if err != nil {
		return err
	}

	distributionFile := "./darwin/Distribution.xml"
	resources := "./darwin/Resources"
	targetFile := b.output + ".pkg"

	b.createDistribution(distributionFile)

	cmdBuild := exec.Command("productbuild",
		"--distribution", distributionFile,
		"--resources", resources,
		"--package-path", tempDest,
		targetFile,
	)

	_, err = cmdBuild.CombinedOutput()
	if err != nil {
		return err
	}

	return nil
}

func (b *Build) createLaunchAgent(rootDir string) {
	launchDir := fmt.Sprintf("%s/Library/LaunchAgents/", rootDir)
	os.MkdirAll(launchDir, 0700)

	launchFile := fmt.Sprintf("%s/com.koding.kite.%s.plist", launchDir, b.appName)

	lFile, err := os.Create(launchFile)
	if err != nil {
		log.Fatalln(err)
	}

	t := template.Must(template.New("launchAgent").Parse(launchAgent))
	t.Execute(lFile, b.appName)

}

func (b *Build) createDistribution(file string) {
	distFile, err := os.Create(file)
	if err != nil {
		log.Fatalln(err)
	}

	t := template.Must(template.New("distribution").Parse(distribution))
	t.Execute(distFile, b.appName)

}

func (b *Build) createScripts(scriptDir string) {
	os.MkdirAll(scriptDir, 0700) // does return nil if exists

	postInstallFile, err := os.Create(scriptDir + "/postInstall")
	if err != nil {
		log.Fatalln(err)
	}
	postInstallFile.Chmod(0755)

	preInstallFile, err := os.Create(scriptDir + "/preInstall")
	if err != nil {
		log.Fatalln(err)
	}
	preInstallFile.Chmod(0755)

	t := template.Must(template.New("postInstall").Parse(postInstall))
	t.Execute(postInstallFile, b.appName)

	t = template.Must(template.New("preInstall").Parse(preInstall))
	t.Execute(preInstallFile, b.appName)
}

func fileExist(dir string) bool {
	var err error
	_, err = os.Stat(dir)
	if err == nil {
		return true // file exist
	}

	if os.IsNotExist(err) {
		return false // file does not exist
	}

	panic(err) // permission errors or something else bad
}
