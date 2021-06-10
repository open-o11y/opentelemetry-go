// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This prerelease script creates a new branch pre_release_<new tag> that
// will contain all release changes, including updated version numbers.
// This is to be used after the verify_release script has successfully
// verified that the versioning of module sets is valid.
//
// TODO: write usage message
// -t

package main

import (
	"fmt"
	"go.opentelemetry.io/otel/internal/tools"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	flag "github.com/spf13/pflag"
)

const (
	defaultVersionsConfigName = "versions"
	defaultVersionsConfigType = "yaml"
)

type config struct {
	versioningFile	string
	moduleSet			string
}

func validateConfig(cfg config) (config, error) {
	if cfg.versioningFile == "" {
		repoRoot, err := tools.FindRepoRoot()
		if err != nil {
			return config{}, fmt.Errorf("no versioning file was given, and could not automatically find repo root: %v", err)
		}
		cfg.versioningFile = filepath.Join(repoRoot,
			fmt.Sprintf("%v.%v", defaultVersionsConfigName, defaultVersionsConfigType))

		fmt.Printf("Using versioning file found at %v\n", cfg.versioningFile)
	}

	if cfg.moduleSet == "" {
		return config{}, fmt.Errorf("required argument module-set was empty")
	}

	return cfg, nil
}

// verifyGitTagsDoNotAlreadyExist checks if Git tags have already been created that match the specific module tag name
// and version number for the modules being updated. If the tag already exists, an error is returned.
func verifyGitTagsDoNotAlreadyExist(newVersion string, modTags []tools.ModuleTagName, coreRepoRoot string) error {
	for _, modTag := range modTags {
		tagSearchString := string(modTag) + "/" + newVersion
		cmd := exec.Command("git", "tag", "-l", tagSearchString)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("could not execute git tag -l %v: %v", tagSearchString, err)
		}
		outputTag := strings.TrimSpace(string(output))
		if outputTag == tagSearchString {
			return fmt.Errorf("git tag already exists for %v", tagSearchString)
		}
	}

	return nil
}

// verifyWorkingTreeClean checks if the working tree is clean (i.e. running 'git diff --exit-code' gives exit code 0).
// If the working tree is not clean, the git diff output is printed, and an error is returned.
func verifyWorkingTreeClean() error {
	cmd := exec.Command("git", "diff", "--exit-code")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("working tree is not clean, can't proceed with the release process:\n\n%v",
			string(output),
		)
	}

	return nil
}

func createPrereleaseBranch(newVersion string) error {
	branchName := "pre_release_" + newVersion
	cmd := exec.Command("git", "checkout", "-b", branchName, "main")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not create new branch %v: %v", branchName, err)
	}
	fmt.Printf("switching to branch %v...\n", branchName)

	return nil
}

// TODO: figure out what to do with version.go file
func updateVersionGo() error {
	return nil
}

func filePathToRegex(fpath string) string {
	replacedSlashes := strings.Replace(fpath, string(filepath.Separator), `\/`, -1)
	replacedPeriods := strings.Replace(replacedSlashes, ".", `\.`, -1)
	return replacedPeriods
}

// updateGoModVersions reads the fromFile (a go.mod file), replaces versions
// for all specified modules in newModPaths, and writes the new go.mod to the toFile file.
func updateGoModVersions(newVersion string, newModPaths []tools.ModulePath, modFilePath tools.ModuleFilePath) error {
	newGoModFile, err := ioutil.ReadFile(string(modFilePath))
	if err != nil {
		panic(err)
	}

	for _, modPath := range newModPaths {
		oldVersionRegex := filePathToRegex(string(modPath)) + ` v[0-9]*\.[0-9]*\.[0-9]`
		r, err := regexp.Compile(oldVersionRegex)
		if err != nil {
			return fmt.Errorf("error compiling regex: %v", err)
		}

		newModVersionString := string(modPath) + " " + newVersion

		newGoModFile = r.ReplaceAll(newGoModFile, []byte(newModVersionString))
	}

	// once all module versions have been updated, overwrite the go.mod file
	ioutil.WriteFile(string(modFilePath), newGoModFile, 0644)

	return nil
}

// updateAllGoModFiles updates ALL modules' requires sections to use the newVersion number
// for the modules given in newModPaths.
func updateAllGoModFiles(newVersion string, newModPaths []tools.ModulePath, modPathMap tools.ModulePathMap) error {
	for _, modFilePath := range modPathMap {
		if err := updateGoModVersions(newVersion, newModPaths, modFilePath); err != nil {
			return fmt.Errorf("could not update module versions in file %v: %v", modFilePath, err)
		}
	}
	return nil
}

// updateGoSum runs 'make lint' to automatically update go.sum files.
func updateGoSum() error {
	cmd := exec.Command("make", "lint")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'make lint' failed: %v", err)
	}

	return nil
}

func commitChanges(newVersion string) error {
	// add changes to git
	cmd := exec.Command("git", "add", ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'git add .' failed: %v", err)
	}

	// make ci
	cmd = exec.Command("make", "ci")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'make ci' failed: %v", err)
	}

	// commit changes to git
	cmd = exec.Command("git", "commit", "-m", "Prepare for releasing " + newVersion)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %v", err)
	}

	return nil
}

func main() {
	// Plain log output, no timestamps.
	log.SetFlags(0)

	cfg := config{}

	flag.StringVarP(&cfg.versioningFile, "versioning-file", "v", "",
		"Path to versioning file that contains definitions of all module sets. " +
			fmt.Sprintf("If unspecified will default to (RepoRoot)/%v.%v",
				defaultVersionsConfigName, defaultVersionsConfigType,),
	)
	flag.StringVarP(&cfg.moduleSet, "module-set", "m", "",
		"Name of module set whose version is being changed. Must be listed in the module set versioning YAML.")
	flag.Parse()

	cfg, err := validateConfig(cfg)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(-1)
	}

	coreRepoRoot, err := tools.FindRepoRoot()
	if err != nil {
		log.Fatalf("unable to find repo root: %v", err)
	}

	// get new version and mod tags to update
	newVersion, newModPaths, newModTags, err := tools.VersionsAndModsToUpdate(cfg.versioningFile, cfg.moduleSet, coreRepoRoot)
	if err != nil {
		log.Fatalf("unable to get modules to update: %v", err)
	}

	if err = verifyGitTagsDoNotAlreadyExist(newVersion, newModTags, coreRepoRoot); err != nil {
			log.Fatalf("verifyGitTagsDoNotAlreadyExist failed: %v", err)
	}

	if err = verifyWorkingTreeClean(); err != nil {
		log.Fatalf("verifyWorkingTreeClean failed: %v", err)
	}

	if err = createPrereleaseBranch(newVersion); err != nil {
		log.Fatalf("createPrereleaseBranch failed: %v", err)
	}

	modPathMap, err := tools.BuildModulePathMap(cfg.versioningFile, coreRepoRoot)

	// TODO: what to do with version.go and references to otel.Version()
	if err = updateVersionGo(); err != nil {
		log.Fatalf("updateVersionGo failed: %v", err)
	}

	if err = updateAllGoModFiles(newVersion, newModPaths, modPathMap); err != nil {
		log.Fatalf("updateAllGoModFiles failed: %v", err)
	}

	if err = updateGoSum(); err != nil {
		log.Fatalf("updateGoSum failed: %v", err)
	}

	if err = commitChanges(newVersion); err != nil {
		log.Fatalf("commitChanges failed: %v", err)
	}

	fmt.Println("\nPrerelease finished successfully.\nNow run the following to verify the changes:\ngit diff main")
	fmt.Println("Then, push the changes to upstream.")
}
