package calisero

import (
	"errors"
	"regexp"
	"runtime"
	"strings"
)

// UserAgent is the product token that opens the User-Agent header of every
// request: the library and its version. A request whose User-Agent starts with
// "Calisero-SMS-Go/" comes from this library.
const UserAgent = "Calisero-SMS-Go/" + Version

// nonTokenCharacters are the characters a header token does not allow.
var nonTokenCharacters = regexp.MustCompile("[^A-Za-z0-9!#$%&'*+.^_`|~-]+")

// buildUserAgent is the User-Agent header: the library and its version, the Go
// release, the operating system and the processor and, when given, your
// application, e.g. "Calisero-SMS-Go/1.0.0 (Go 1.27.1; linux amd64) MyShop/2.1.0".
func buildUserAgent(appProduct string) string {
	userAgent := UserAgent + " (" + runtimeComment(runtime.Version(), runtime.GOOS, runtime.GOARCH) + ")"

	if appProduct != "" {
		userAgent += " " + appProduct
	}

	return userAgent
}

// runtimeComment describes the Go release, the operating system and the
// processor, e.g. "Go 1.27.1; linux amd64".
func runtimeComment(goVersion, goos, goarch string) string {
	return "Go " + goRelease(goVersion) + "; " + commentSafe(goos) + " " + commentSafe(goarch)
}

// goRelease is the release runtime.Version() names: "1.27.1" for "go1.27.1",
// "1.28-a1b2c3d4" for a development build's "devel go1.28-a1b2c3d4 …"; "devel"
// when it names none.
func goRelease(version string) string {
	for _, field := range strings.Fields(version) {
		if len(field) > 2 && strings.HasPrefix(field, "go") && field[2] >= '0' && field[2] <= '9' {
			return commentSafe(field[2:])
		}
	}

	return "devel"
}

// commentSafe reduces a value to characters that cannot break the header's comment.
func commentSafe(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune(".+_-", r):
			return r
		default:
			return '-'
		}
	}, value)
}

// appProduct is your application as a product token, "name/version", both reduced
// to the characters a header token allows, so the header stays a single valid line.
func appProduct(name, version string) (string, error) {
	name = toToken(name)

	if name == "" {
		return "", errors.New("calisero: the application name must hold letters, digits or -._ characters")
	}

	if version = toToken(version); version != "" {
		return name + "/" + version, nil
	}

	return name, nil
}

func toToken(value string) string {
	return strings.Trim(nonTokenCharacters.ReplaceAllString(value, "-"), "-")
}
