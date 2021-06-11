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

// This prerelease script creates a new branch pre_release_<module set name>_<new tag> that
// will contain all release changes, including updated version numbers.
// This is to be used after the verify_release script has successfully
// verified that the versioning of module sets is valid.

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
	VersioningFile     string
	ModuleSet          string
	FromExistingBranch string
	SkipMakeLint	   bool
}

func validateConfig(cfg config) (config, error) {
	if cfg.VersioningFile == "" {
		repoRoot, err := tools.FindRepoRoot()
		if err != nil {
			return config{}, fmt.Errorf("no versioning file was given, and could not automatically find repo root: %v", err)
		}
		cfg.VersioningFile = filepath.Join(repoRoot,
			fmt.Sprintf("%v.%v", defaultVersionsConfigName, defaultVersionsConfigType))

		fmt.Printf("Using versioning file found at %v\n", cfg.VersioningFile)
	}

	if cfg.ModuleSet == "" {
		return config{}, fmt.Errorf("required argument module-set was empty")
	}

	if cfg.FromExistingBranch == "" {
		// get current branch
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		output, err := cmd.Output()
		if err != nil {
			return config{}, fmt.Errorf("could not get current branch: %v", err)
		}

		cfg.FromExistingBranch = strings.TrimSpace(string(output))
	}

	return cfg, nil
}

// verifyGitTagsDoNotAlreadyExist checks if Git tags have already been created that match the specific module tag name
// and version number for the modules being updated. If the tag already exists, an error is returned.
func verifyGitTagsDoNotAlreadyExist(version string, modTagNames []tools.ModuleTagName, coreRepoRoot string) error {
	modFullTags := tools.CombineModuleTagNamesAndVersion(modTagNames, version)

	for _, newFullTag := range modFullTags {
		cmd := exec.Command("git", "tag", "-l", newFullTag)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("could not execute git tag -l %v: %v", newFullTag, err)
		}

		outputTag := strings.TrimSpace(string(output))
		if outputTag == newFullTag {
			return fmt.Errorf("git tag already exists for %v", newFullTag)
		}
	}

	return nil
}

// verifyWorkingTreeClean checks if the working tree is clean (i.e. running 'git diff --exit-code' gives exit code 0).
// If the working tree is not clean, the git diff output is printed, and an error is returned.
func verifyWorkingTreeClean() error {
	cmd := exec.Command("git", "diff", "--exit-code")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("working tree is not clean, can't proceed with the release process:\n\n%v",
			string(output),
		)
	}

	return nil
}

func createPrereleaseBranch(modSet, newVersion, fromExistingBranch string) error {
	branchNameElements := []string{"pre_release", modSet, newVersion}
	branchName := strings.Join(branchNameElements, "_")
	fmt.Printf("git checkout -b %v %v\n", branchName, fromExistingBranch)
	cmd := exec.Command("git", "checkout", "-b", branchName, fromExistingBranch)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not create new branch %v: %v (%v)", branchName, string(output), err)
	}

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
	fmt.Println("Updating all module versions in go.mod files...")
	for _, modFilePath := range modPathMap {
		if err := updateGoModVersions(newVersion, newModPaths, modFilePath); err != nil {
			return fmt.Errorf("could not update module versions in file %v: %v", modFilePath, err)
		}
	}
	return nil
}

// runMakeLint runs 'make lint' to automatically update go.sum files.
func runMakeLint() error {
	fmt.Println("Updating go.sum with 'make lint'...")

	cmd := exec.Command("make", "lint")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("'make lint' failed: %v (%v)", string(output), err)
	}

	return nil
}

func commitChanges(newVersion string) error {
	// add changes to git
	cmd := exec.Command("git", "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("'git add .' failed: %v (%v)", string(output), err)
	}

	// make ci
	cmd = exec.Command("make", "ci")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("'make ci' failed: %v (%v)", string(output), err)
	}

	// commit changes to git
	cmd = exec.Command("git", "commit", "-m", "Prepare for releasing "+newVersion)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v (%v)", string(output), err)
	}

	return nil
}

func main() {
	// Plain log output, no timestamps.
	log.SetFlags(0)

	cfg := config{}

	flag.StringVarP(&cfg.VersioningFile, "versioning-file", "v", "",
		"Path to versioning file that contains definitions of all module sets. "+
			fmt.Sprintf("If unspecified will default to (RepoRoot)/%v.%v",
				defaultVersionsConfigName, defaultVersionsConfigType),
	)
	flag.StringVarP(&cfg.ModuleSet, "module-set", "m", "",
		"Name of module set whose version is being changed. Must be listed in the module set versioning YAML.",
	)
	flag.StringVarP(&cfg.FromExistingBranch, "from-existing-branch", "f", "",
		"Name of existing branch from which to base the pre-release branch. If unspecified, defaults to current branch.",
	)
	flag.BoolVarP(&cfg.SkipMakeLint, "skip-make-lint", "s", false,
		"Specify this flag to skip the 'make lint' used to update go.sum files automatically. " +
		"To be used for debugging purposes. Should not be skipped during actual release.",
	)
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

	fmt.Println("Changing to root directory...")
	os.Chdir(coreRepoRoot)

	// get new version and mod tags to update
	newVersion, newModPaths, newModTagNames, err := tools.VersionsAndModulesToUpdate(cfg.VersioningFile, cfg.ModuleSet, coreRepoRoot)
	if err != nil {
		log.Fatalf("unable to get modules to update: %v", err)
	}

	fmt.Println("Checking for tags", newModTagNames)

	if err = verifyGitTagsDoNotAlreadyExist(newVersion, newModTagNames, coreRepoRoot); err != nil {
		log.Fatalf("verifyGitTagsDoNotAlreadyExist failed: %v", err)
	}

	if err = verifyWorkingTreeClean(); err != nil {
		log.Fatalf("verifyWorkingTreeClean failed: %v", err)
	}

	if err = createPrereleaseBranch(cfg.ModuleSet, newVersion, cfg.FromExistingBranch); err != nil {
		log.Fatalf("createPrereleaseBranch failed: %v", err)
	}

	modPathMap, err := tools.BuildModulePathMap(cfg.VersioningFile, coreRepoRoot)

	// TODO: what to do with version.go and references to otel.Version()
	if err = updateVersionGo(); err != nil {
		log.Fatalf("updateVersionGo failed: %v", err)
	}

	if err = updateAllGoModFiles(newVersion, newModPaths, modPathMap); err != nil {
		log.Fatalf("updateAllGoModFiles failed: %v", err)
	}

	if !cfg.SkipMakeLint {
		if err = runMakeLint(); err != nil {
			log.Fatalf("runMakeLint failed: %v", err)
		}
	}

	if err = commitChanges(newVersion); err != nil {
		log.Fatalf("commitChanges failed: %v", err)
	}

	fmt.Println("\nPrerelease finished successfully.\nNow run the following to verify the changes:\ngit diff main")
	fmt.Println("Then, push the changes to upstream.")
}
