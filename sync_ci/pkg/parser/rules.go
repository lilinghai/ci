package parser

import (
	"regexp"
	"sort"
	"strings"
)

var envRules = map[string]string{
	"plugin_ver_mismatch": "[FATAL].*?plugin was built with a different version of package",
	"dns_resolve_failure": "Could not resolve host",
	"pod_vanish":          "\\[get\\]  for kind: \\[Pod\\]  with name: \\[(.*?)\\]  in namespace: \\[jenkins-ci\\]  failed",
	"http_500":            "500 Internal Server Error",
	"kill_signal":         "signal killed|signal interrupt",
	"core_dumped":         "core dumped",
	"rewrite_error":       "Rewrite error",
	"connection_closed":   "java\\.nio\\.channels\\.ClosedByInterruptException",
	"connection_reset":    "[Cc]onnection reset",
	"socket_timeout":      "java.net.SocketTimeoutException",
	"socket_close":        "java.net.SocketException: Socket closed",
}
var envParsers = []parser{
	&envParser{envRules},
}
var caseParser = []parser{
	&tidbUtParser{[]string{"tidb_ghpr_unit_test", "tidb_ghpr_check", "tidb_ghpr_check_2"}},
}

type parser interface {
	Parse(job string, lines []string) []string
}

type tidbUtParser struct {
	jobs []string
}

func (t *tidbUtParser) Parse(job string, lines []string) []string {
	var res []string
	pattern := `FAIL:|PANIC:|WARNING: DATA RACE`
	r, _ := regexp.Compile(pattern)
	if len(t.jobs) == sort.SearchStrings(t.jobs, job) {
		return res
	}
	if strings.Contains(lines[0], "FAIL: TestT") {
		return res
	}
	matchedStr := r.FindString(lines[0])
	if len(matchedStr) == 0 {
		return res
	}
	if matchedStr == "WARNING: DATA RACE" {
		failLine := strings.TrimSpace(lines[2])
		failDetail := strings.Join([]string{"DATA RACE", strings.Split(failLine, " ")[1]}, ":")
		res = append(res, failDetail)
		return res
	}
	failLine := strings.TrimSpace(lines[0])
	failCodePosition := strings.Split(
		strings.Split(failLine, " ")[2], ":")[0]
	failDetail := strings.Join([]string{failCodePosition, strings.Split(failLine, " ")[3]}, ":")
	res = append(res, failDetail)
	return res
}

type envParser struct {
	rules map[string]string
}

func (t *envParser) Parse(job string, lines []string) []string {
	var res []string
	for rule, pattern := range t.rules {
		matched, _ := regexp.MatchString(pattern, lines[0])
		if matched {
			res = append(res, rule)
		}
	}
	return res
}