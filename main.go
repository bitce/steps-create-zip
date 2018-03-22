package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-steputils/input"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

type config struct {
	SourcePath  string `env:"source_path,dir"`
	Destination string `env:"destionation"`
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}
	cfg.print()

	_, err := ensureZIPExtension(cfg.SourcePath, cfg.Destination)

	if err != nil {
		failf("Issue with compress: %s", err)
		return
	}
}

func ensureZIPExtension(sourcePath string, destionation string) (string, error) {
	if !strings.HasPrefix(destionation, ".zip") {
		destionation += ".zip"
	}

	ensureDir(destionation)

	zipfile, err := os.Create(destionation)
	if err != nil {
		return "", err
	}

	defer func() {
		if err = zipfile.Close(); err != nil {
			log.Errorf("%s", err)
		}
	}()

	archive := zip.NewWriter(zipfile)

	defer func() {
		if err = archive.Close(); err != nil {
			log.Errorf("%s", err)
		}
	}()

	info, err := os.Lstat(sourcePath)
	if err != nil {
		return "", err
	}

	var baseDir string
	if !info.IsDir() {
		baseDir = filepath.Base(sourcePath)
	}

	err = filepath.Walk(sourcePath, func(pth string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		isSymlink, evaledPath, err := checkSymlink(pth)
		if err != nil {
			return err
		}

		originalPath := pth

		if isSymlink {
			pth = evaledPath
		}

		info, err = os.Lstat(pth)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			if isSymlink {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(originalPath, sourcePath))
			} else {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(pth, sourcePath))
			}

		} else {
			baseDir = "/"
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(pth)
		if err != nil {
			return err
		}

		defer func() {
			if err = file.Close(); err != nil {
				log.Errorf("%s", err)
			}
		}()

		_, err = io.Copy(writer, file)
		return err
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprint(zipfile), nil
}

// If the file is a symbolic link it will evaulate the path name.

// If the target is a symbolic link it will return true, the evaulated path, and the original path.
// If the target is  not a symbolic link it will return false
// If there is an error it will return the error.
func checkSymlink(path string) (isSymlink bool, evaledPath string, error error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, "", err
	}

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		evaledPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return true, "", err
		}

		return true, evaledPath, nil
	}
	return false, "", nil
}

// ensureDir will check the target path's existence.
// If the destionation does not exist it will create that.
func ensureDir(destionation string) {
	dirOftargetPath := filepath.Dir(destionation)

	exits, err := pathutil.IsDirExists(dirOftargetPath)
	if err != nil {
		log.Errorf("Failed to open %s, error: %s", dirOftargetPath, err)
	}

	if exits == false {
		fmt.Println()
		log.Warnf("targetRootPath: %s does not exist", dirOftargetPath)
		err = os.MkdirAll(dirOftargetPath, os.ModePerm)
		if err != nil {
			log.Errorf("Failed to create directory, error: %s", err)
		}
	}

	checkAlreadyExist(destionation)
}

func (configs config) print() {
	log.Infof("Create ZIP configs:")
	log.Printf("- SourcePath: %s", configs.SourcePath)
	log.Printf("- TargetDir: %s", configs.Destination)
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

// It will warn the user if the zip has already exist at the destionation.
// Just log a warning, still continues.
func checkAlreadyExist(destionation string) {
	targetPathWithExtension := destionation

	if !strings.HasPrefix(targetPathWithExtension, ".zip") {
		targetPathWithExtension += ".zip"
	}
	splittedTargetPathWithExtension := strings.Split(targetPathWithExtension, "/")

	targetName := targetPathWithExtension
	if len(splittedTargetPathWithExtension) > 0 {
		targetName = splittedTargetPathWithExtension[len(splittedTargetPathWithExtension)-1]
	}

	if err := input.ValidateIfPathExists(targetPathWithExtension); err == nil {
		targetAlreadyExist := fmt.Sprintf("The %s already exists at location: %s ", targetName, targetPathWithExtension)

		log.Warnf(targetAlreadyExist)
		fmt.Println()
	}
}
