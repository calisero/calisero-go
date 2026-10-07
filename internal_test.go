package calisero

import (
	"context"
	"math"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	// The tests compare the library's own rules with the time zone database, which
	// must be there whatever the machine
	_ "time/tzdata"
)

func TestGoReleaseReadsTheReleaseOfRuntimeVersion(t *testing.T) {
	cases := []struct{ version, release string }{
		{"go1.27.1", "1.27.1"},
		{"go1.22.0", "1.22.0"},
		{"go1.27.1 X:nocoverageredesign", "1.27.1"},
		{"devel go1.28-a1b2c3d4 Tue Oct 6 10:00:00 2026 +0000", "1.28-a1b2c3d4"},
		{"go1.28rc1", "1.28rc1"},
		{"go1.2(beta)", "1.2-beta-"},
		{"", "devel"},
		{"devel +a1b2c3d4", "devel"},
		{"gopher", "devel"},
	}

	for _, tc := range cases {
		if got := goRelease(tc.version); got != tc.release {
			t.Errorf("goRelease(%q) = %q, want %q", tc.version, got, tc.release)
		}
	}
}

func TestSendRefusesABodyThatCannotBeEncoded(t *testing.T) {
	client, err := NewClient("test-api-key", WithBaseURL("http://127.0.0.1:9/api/v1"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.send(context.Background(), http.MethodPost, "/messages", nil, map[string]any{"amount": math.Inf(1)})

	if err == nil || !strings.HasPrefix(err.Error(), "calisero: encode the request body: ") {
		t.Fatalf("got %v", err)
	}
}

func TestRuntimeCommentDescribesTheGoReleaseAndThePlatform(t *testing.T) {
	if got := runtimeComment("go1.27.1", "linux", "amd64"); got != "Go 1.27.1; linux amd64" {
		t.Errorf("got %q", got)
	}

	if got := runtimeComment("go1.27.1", "dar(win)", "arm 64"); got != "Go 1.27.1; dar-win- arm-64" {
		t.Errorf("a platform that would break the comment: got %q", got)
	}
}

func TestBuildUserAgentAppendsTheApplication(t *testing.T) {
	if got, want := buildUserAgent("MyShop/2.1.0"), UserAgent+" ("+runtimeCommentOfThisProcess()+") MyShop/2.1.0"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if got, want := buildUserAgent(""), UserAgent+" ("+runtimeCommentOfThisProcess()+")"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func runtimeCommentOfThisProcess() string {
	return runtimeComment(runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func TestRomaniaOffsetFollowsTheTimeZoneDatabase(t *testing.T) {
	location, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Fatalf("the time zone database is not available: %v", err)
	}

	// Every quarter of an hour from 2020 to 2035, the changes of time included
	for instant := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC); instant.Year() < 2036; instant = instant.Add(15 * time.Minute) {
		_, want := instant.In(location).Zone()

		if got := romaniaOffset(instant); got != want {
			t.Fatalf("romaniaOffset(%s) = %d, want %d", instant, got, want)
		}
	}
}

func TestLoadLocation(t *testing.T) {
	if location := loadLocation("Europe/Bucharest"); location == nil || location.String() != "Europe/Bucharest" {
		t.Errorf("Europe/Bucharest: got %v", location)
	}

	if location := loadLocation("Nowhere/Atlantis"); location != nil {
		t.Errorf("a zone the database does not have: got %v", location)
	}

	if romaniaLocation() == nil {
		t.Error("the tests run with a time zone database: Romania time should come from it")
	}
}

func TestInRomaniaWithoutTheTimeZoneDatabase(t *testing.T) {
	cases := map[time.Time]string{
		time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC):   "2026-07-15 12:30:00 EEST",
		time.Date(2026, 1, 15, 9, 30, 5, 0, time.UTC):   "2026-01-15 11:30:05 EET",
		time.Date(2026, 3, 29, 0, 59, 59, 0, time.UTC):  "2026-03-29 02:59:59 EET",
		time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC):    "2026-03-29 04:00:00 EEST",
		time.Date(2026, 10, 25, 0, 59, 59, 0, time.UTC): "2026-10-25 03:59:59 EEST",
		time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC):   "2026-10-25 03:00:00 EET",
	}

	for instant, want := range cases {
		if got := inRomania(instant, nil).Format("2006-01-02 15:04:05 MST"); got != want {
			t.Errorf("inRomania(%s, nil) = %s, want %s", instant, got, want)
		}
	}
}

func TestToIntReadsIntegersHeldExactly(t *testing.T) {
	cases := []struct {
		value any
		want  int
		ok    bool
	}{
		{float64(42), 42, true},
		{float64(0), 0, true},
		{float64(-7), -7, true},
		{1.0, 1, true},
		{float64(maxSafeInteger), maxSafeInteger, true},
		{float64(maxSafeInteger + 1), 0, false},
		{1.5, 0, false},
		{math.NaN(), 0, false},
		{math.Inf(1), 0, false},
		{"5", 0, false},
		{true, 0, false},
		{nil, 0, false},
	}

	for _, tc := range cases {
		got, ok := toInt(tc.value)

		if got != tc.want || ok != tc.ok {
			t.Errorf("toInt(%#v) = %d, %t; want %d, %t", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}

func TestToNumberReadsNumbersAndNumericStrings(t *testing.T) {
	cases := []struct {
		value any
		want  float64
		ok    bool
	}{
		{0.0378, 0.0378, true},
		{"100.50", 100.5, true},
		{" 3 ", 3, true},
		{"-0.5", -0.5, true},
		{"", 0, false},
		{"abc", 0, false},
		{"Inf", 0, false},
		{"NaN", 0, false},
		{"1e400", 0, false},
		{true, 0, false},
		{nil, 0, false},
		{[]any{}, 0, false},
	}

	for _, tc := range cases {
		got, ok := toNumber(tc.value)

		if got != tc.want || ok != tc.ok {
			t.Errorf("toNumber(%#v) = %v, %t; want %v, %t", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseTimeReadsTheFormatsOfTheAPI(t *testing.T) {
	cases := map[string]time.Time{
		"2025-02-06T10:18:43.000000Z": time.Date(2025, 2, 6, 10, 18, 43, 0, time.UTC),
		"2025-02-06T10:18:43Z":        time.Date(2025, 2, 6, 10, 18, 43, 0, time.UTC),
		"2025-02-06T10:18:43.123456Z": time.Date(2025, 2, 6, 10, 18, 43, 123456000, time.UTC),
		"2026-10-01T00:00:00+03:00":   time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC),
		"2025-02-06 10:18:43":         time.Date(2025, 2, 6, 10, 18, 43, 0, time.UTC),
		"2025-02-06T10:18:43.5-02:30": time.Date(2025, 2, 6, 12, 48, 43, 500000000, time.UTC),
		"2024-02-29T23:59:59.999999Z": time.Date(2024, 2, 29, 23, 59, 59, 999999000, time.UTC),
		"2025-12-31T22:00:00.000000Z": time.Date(2025, 12, 31, 22, 0, 0, 0, time.UTC),
	}

	for text, want := range cases {
		got, ok := parseTime(text)

		if !ok || !got.Equal(want) {
			t.Errorf("parseTime(%q) = %s, %t; want %s", text, got, ok, want)
		}
	}

	for _, text := range []string{"", "yesterday", "2025-02-06", "06/02/2025 10:18", "2025-02-30T10:00:00Z"} {
		if _, ok := parseTime(text); ok {
			t.Errorf("parseTime(%q) read a date-time", text)
		}
	}
}

func TestDescribeNamesWhatAValueIs(t *testing.T) {
	cases := []struct {
		value   any
		present bool
		want    string
	}{
		{nil, false, "nothing"},
		{nil, true, "null"},
		{"text", true, "a string"},
		{1.5, true, "a number"},
		{false, true, "a boolean"},
		{[]any{}, true, "an array"},
		{map[string]any{}, true, "an object"},
		{42, true, "a int"},
	}

	for _, tc := range cases {
		if got := describe(tc.value, tc.present); got != tc.want {
			t.Errorf("describe(%#v, %t) = %q, want %q", tc.value, tc.present, got, tc.want)
		}
	}
}

func TestReaderKeepsTheFirstError(t *testing.T) {
	r := newReader(map[string]any{"name": 42.0, "count": "many"}, "thing")

	if r.str("name") != "" || r.integer("count") != 0 || r.boolean("flag") || r.number("amount") != 0 {
		t.Fatal("a read after an error returned a value")
	}

	if r.object("nested") != nil || r.array("items") != nil || r.optArray("items") != nil || r.optStr("name") != "" {
		t.Fatal("a read after an error returned a value")
	}

	if r.optInt("count") != nil || r.optNumber("amount") != nil || r.optTimestamp("at") != nil || !r.timestamp("at").IsZero() {
		t.Fatal("a read after an error returned a value")
	}

	if r.nonEmpty("name") != "" {
		t.Fatal("a read after an error returned a value")
	}

	if r.err == nil || r.err.Error() != `invalid thing: "name" must be a string, got a number` {
		t.Fatalf("got the error %v", r.err)
	}
}

func TestHeaderStringMatchesNamesWhateverTheirCase(t *testing.T) {
	header := http.Header{"x-trace-id": {" " + "trace-1" + " "}, "X-Empty": {}}

	if got := headerString(header, "X-Trace-Id"); got != "trace-1" {
		t.Errorf("a lowercase name: got %q", got)
	}

	if got := headerString(header, "X-Empty"); got != "" {
		t.Errorf("a header without values: got %q", got)
	}

	if got := headerString(nil, "X-Trace-Id"); got != "" {
		t.Errorf("no headers: got %q", got)
	}
}

func TestHeaderIntReadsNonNegativeIntegersOnly(t *testing.T) {
	cases := map[string]*int{
		"17":                            intPointer(17),
		" 240 ":                         intPointer(240),
		"0":                             intPointer(0),
		"-1":                            nil,
		"1.5":                           nil,
		"abc":                           nil,
		"":                              nil,
		"Wed, 21 Oct 2026 07:28:00 GMT": nil,
		"99999999999999999999":          nil,
	}

	for value, want := range cases {
		got := headerInt(http.Header{"Retry-After": {value}}, "Retry-After")

		if (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Errorf("headerInt(%q) = %v, want %v", value, got, want)
		}
	}
}

func intPointer(value int) *int {
	return &value
}
