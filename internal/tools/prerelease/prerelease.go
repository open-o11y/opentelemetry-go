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
	"path/filepath"

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

func verifyGitTagsDoNotAlreadyExist() error {
	return nil
}

func verifyWorkingTreenClean() error {
	return nil
}

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

	modSetMap, err := common.BuildModuleSetsMap(cfg.versioningFile)
	if err != nil {
		log.Fatalf("unable to build module sets map: %v", err)
	}

	newVersion, modsToUpdate := modSetMap[cfg.moduleSet]
	fmt.Println(newVersion, modsToUpdate)

	// check if git tag already exists for any module listed in the set
	if err = verifyGitTagsDoNotAlreadyExist(); err != nil {
			log.Fatalf("verifyGitTagsDoNotAlreadyExist failed: %v", err)
	}

	// check if working tree is clean (if not, can't proceed with release process)
	if err = verifyWorkingTreenClean(); err != nil {
		log.Fatalf("verifyWorkingTreenClean failed: %v", err)
	}

	// what to do with version.go?

	// create branch pre_release_<TAG>
	// find all go.mod files
	// update all go.mod dependencies to use new versions
	// TODO: figure out how to update module path for semantic import versioning
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

	fmt.Println("Now run the following to verify the changes:\ngitdiffmain")
	fmt.Println("Then, push the changes to upstream.")
}
