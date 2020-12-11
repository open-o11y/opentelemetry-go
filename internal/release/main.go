package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	projectRoot := os.Getenv("GITHUB_WORKSPACE")
	newTag := os.Getenv("RELEASE_TAG")

	if len(projectRoot) == 0 {
		log.Fatal("$GITHUB_WORKSPACE is empty")
	}

	if len(newTag) == 0 {
		log.Fatal("$RELEASE_TAG is empty")
	}
	newTag = newTag[1:] //remove 'v' from 'v1.2.0'
	file, err := os.OpenFile(projectRoot+"/CHANGELOG.md", os.O_RDONLY, os.ModeExclusive)
	if err != nil {
		log.Fatalf("failed opening changelog: %s", err)
	}
	if err != nil {
		log.Fatalf("failed compilign regex: %s", err)
	}

	scanner := bufio.NewScanner(file)
	newChangelog := make([]string, 0)
	for scanner.Scan() {
		var sb strings.Builder
		currLine := scanner.Text()
		if strings.Contains(currLine, "## [Unreleased]") {
			temp := fmt.Sprintf("## [%s] - %s\n", newTag, time.Now().Format("2006-01-30"))
			fmt.Fprintf(&sb, "%s\n\n", currLine)
			newChangelog = append(newChangelog, sb.String())
			newChangelog = append(newChangelog, temp)
		} else if strings.Contains(currLine, "[Unreleased]") {
			newUnreleasedLine := fmt.Sprintf("[Unreleased]: https://github.com/open-telemetry/opentelemetry-go/compare/v%s...HEAD\n", newTag)
			newReleaseLine := fmt.Sprintf("[%s]: https://github.com/open-telemetry/opentelemetry-go/releases/tag/v%s\n", newTag, newTag)
			newChangelog = append(newChangelog, newUnreleasedLine)
			newChangelog = append(newChangelog, newReleaseLine)
		} else {
			fmt.Fprintf(&sb, "%s\n", currLine)
			newChangelog = append(newChangelog, sb.String())
		}
	}
	file.Close()
	file, err = os.OpenFile(projectRoot+"/CHANGELOG.md", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModeExclusive)
	if err != nil {
		log.Fatalf("failed opening changelog for write: %s", err)
	}
	defer file.Close()
	for _, line := range newChangelog {
		file.WriteString(line)
	}
}
