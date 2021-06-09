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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	flag "github.com/spf13/pflag"

	"go.opentelemetry.io/otel/internal/tools/common"
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
		repoRoot, err := common.FindRepoRoot()
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
func verifyGitTagsDoNotAlreadyExist(newVersion string, modTags []common.ModuleTagName, coreRepoRoot string) error {
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
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not create new branch %v: %v", branchName, err)
	}
	fmt.Println(output)
	return nil
}

func updateVersionGo() error {
	return nil
}

// find all go.mod files
// update all go.mod dependencies to use new versions
// TODO: figure out how to update module path for semantic import versioning
func updateGoModFiles() error {

	return nil
}

func updateGoSum() error {
	return nil
}

func commitChanges() error {
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

	coreRepoRoot, err := common.FindRepoRoot()
	if err != nil {
		log.Fatalf("unable to find repo root: %v", err)
	}

	// get new version and mod tags to update
	newVersion, newModTags, err := common.VersionsAndModsToUpdate(cfg.versioningFile, cfg.moduleSet, coreRepoRoot)
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

	// TODO: what to do with version.go and references to otel.Version()
	if err = updateVersionGo(); err != nil {
		log.Fatalf("updateVersionGo failed: %v", err)
	}

	if err = updateGoModFiles(); err != nil {
		log.Fatalf("updateGoModFiles failed: %v", err)
	}

	//# Run lint to update go.sum
	//make lint
	if err = updateGoSum(); err != nil {
		log.Fatalf("updateGoSum failed: %v", err)
	}

	//# Add changes and commit.
	//	git add .
	//	make ci
	//git commit -m "Prepare for releasing $TAG"
	if err = commitChanges(); err != nil {
		log.Fatalf("commitChanges failed: %v", err)
	}

	fmt.Println("\nPrerelease finished successfully.\nNow run the following to verify the changes:\ngit diff main")
	fmt.Println("Then, push the changes to upstream.")
}
