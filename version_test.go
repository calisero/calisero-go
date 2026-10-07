package calisero_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	calisero "github.com/calisero/calisero-go"
)

func TestVersionMatchesTheLatestVersionOfTheChangelog(t *testing.T) {
	changelog, err := os.ReadFile("CHANGELOG.md")
	noError(t, err)

	latest := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`).FindStringSubmatch(string(changelog))
	if latest == nil {
		t.Fatal("CHANGELOG.md names no released version")
	}

	equal(t, calisero.Version, latest[1], "Version")
}

func TestVersionIsASemanticVersion(t *testing.T) {
	equal(t, regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(calisero.Version), true, "Version "+calisero.Version)
}

func TestTheUserAgentNamesTheVersion(t *testing.T) {
	client, err := calisero.NewClient("test-api-key")
	noError(t, err)

	equal(t, strings.HasPrefix(client.UserAgent(), "Calisero-SMS-Go/"+calisero.Version+" "), true, "User-Agent "+client.UserAgent())
}
