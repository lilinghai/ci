package parser

import (
	"regexp"
	"strings"
)

var caseParsers = []parser{
	&tidbUtParser{map[string]bool{"tidb_ghpr_unit_test": true, "tidb_ghpr_check": true, "tidb_ghpr_check_2": true}},
	&integrationTestParser{},
	&tikvUtParser{},
}

var compileParsers = []parser{
	&simpleParser{rules: []rule{
		{name: "rewrite error", patterns: []string{"Rewrite error"}},
		{name: "go.mod error", patterns: []string{"go: errors parsing go.mod"}},
		{name: "plugin error", patterns: []string{"compile plugin source code failure"}},
		{name: "syntax error", patterns: []string{"syntax error:"}},
		{name: "build failpoint error", patterns: []string{`make: \*\*\* \[failpoint-enable\] Error`}},
		{jobs: []string{"tidb_ghpr_check"}, name: "server_check build error", patterns: []string{`make: \*\*\* \[server_check\] Error`}},
		{jobs: []string{"tidb_ghpr_check_2"}, name: "replace parser error", patterns: []string{`replace.*github.com/pingcap/parser`}},
		{jobs: []string{"tidb_ghpr_check_2"}, name: "build error", patterns: []string{`\[build failed\]`}},
		{jobs: []string{"tidb_ghpr_build"}, name: "build error", patterns: []string{`make: \*\*\* \[(server|importer)\] Error`}},
		{jobs: []string{"tikv_ghpr_test", "tikv_ghpr_integration_common_test"}, name: "build error", patterns: []string{`error: could not compile`}},
	}},
}

var checkParsers = []parser{
	&simpleParser{rules: []rule{
		{jobs: []string{"tidb_ghpr_check"}, name: "check error", patterns: []string{`make: \*\*\* \[(fmt|errcheck|unconvert|lint|tidy|testSuite|check-static|vet|staticcheck|errdoc|checkdep|gogenerate)\] Error`}},
		{jobs: []string{"tikv_ghpr_test"}, name: "check error", patterns: []string{`Please make format and run tests before creating a PR`, `make: \*\*\* \[(fmt|clippy)\] Error`}},
	}},
	&tidbCheckParser{},
}

type parser interface {
	parse(job string, lines []string) []string
}

// if job is empty , it matches all jobs
type rule struct {
	jobs     []string
	name     string
	patterns []string
}
type simpleParser struct {
	rules []rule
}

func (s *simpleParser) parse(job string, lines []string) []string {
	var res []string
	for _, r := range s.rules {
		matched := len(r.jobs) == 0
		for _, j := range r.jobs {
			if j == job {
				matched = true
			}
		}
		if !matched {
			continue
		}
		for _, p := range r.patterns {
			matched, _ = regexp.MatchString(p, lines[0])
			if matched {
				res = append(res, r.name)
				break
			}
		}
	}
	return res
}

type tidbCheckParser struct {
}

func (t *tidbCheckParser) parse(job string, lines []string) []string {
	var res []string
	if job == "tidb_ghpr_check" {
		r := regexp.MustCompile(`FATAL.*?error=.*`)
		matchedStr := r.FindString(lines[0])
		if len(matchedStr) > 0 && !strings.Contains(lines[0], "open DB failed") {
			res = append(res, matchedStr)
		}
	}
	return res
}

type tidbUtParser struct {
	jobs map[string]bool
}

func (t *tidbUtParser) parse(job string, lines []string) []string {
	var res []string
	pattern := `FAIL:|panic: runtime error:.*|panic: test timed out|WARNING: DATA RACE|leaktest.go.* Test .* check-count .* appears to have leaked: .*`
	r := regexp.MustCompile(pattern)
	if _, ok := t.jobs[job]; !ok {
		return res
	}
	matchedStr := r.FindString(lines[0])
	if len(matchedStr) == 0 {
		return res
	}
	if strings.Contains(matchedStr, "leaktest.go") {
		failLine := strings.TrimSpace(lines[0])
		prefix := regexp.MustCompile(`leaktest.go:[0-9]*:`).FindAllString(failLine, -1)[0]
		failDetail := strings.Join([]string{strings.Split(prefix, ":")[0], strings.TrimSpace(strings.Split(strings.Split(failLine, prefix)[1], "(0x")[0])}, ":")
		res = append(res, failDetail)
		return res
	}
	if matchedStr == "WARNING: DATA RACE" {
		failLine := strings.TrimSpace(lines[2])
		failDetail := strings.Join([]string{"DATA RACE", regexp.MustCompile(`[^\s]+`).FindAllString(failLine, -1)[1]}, ":")
		res = append(res, failDetail)
		return res
	}
	if strings.Contains(strings.ToLower(matchedStr), "panic:") {
		res = append(res, matchedStr)
		return res
	}
	//parse func fail
	if strings.Contains(lines[0], "FAIL: TestT") {
		return res
	}
	failLine := strings.TrimSpace(lines[0])
	failCodePosition := strings.Split(
		strings.Split(failLine, " ")[2], ":")[0]
	failDetail := strings.Join([]string{failCodePosition, strings.Split(failLine, " ")[3]}, ":")
	res = append(res, failDetail)
	return res
}

type integrationTestParser struct {
}

//TODO require other rules
func (t *integrationTestParser) parse(job string, lines []string) []string {
	var res []string
	r := regexp.MustCompile(`level=fatal msg=.*`)
	matchedStr := r.FindString(lines[0])
	if len(matchedStr) != 0 {
		if matched, title := MatchAndParseSQLStmtTest(matchedStr); matched {
			res = append(res, title)
		} else {
			res = append(res, matchedStr)
		}
		return res
	}

	r = regexp.MustCompile(`Test fail: Outputs are not matching`)
	if len(r.FindString(lines[0])) != 0 {
		detail := strings.TrimSpace(strings.Split(lines[1], "Test case:")[1])
		res = append(res, detail)
		return res
	}
	if job == "tidb_ghpr_tics_test" {
		r = regexp.MustCompile(`Error:|Result:`)
		if len(r.FindString(lines[0])) != 0 && len(r.FindString(lines[1])) != 0 {
			res = append(res, strings.TrimSpace(strings.Split(lines[0], "Error:")[1]))
		}
	}
	return res
}

type tikvUtParser struct {
}

func (t *tikvUtParser) parse(job string, lines []string) []string {
	var res []string
	if job != "tikv_ghpr_test" {
		return res
	}
	if strings.Contains(lines[0], "there is a core dumped, which should not happen") {
		res = append(res, "core dumped")
		return res
	}
	startMatchedStr := regexp.MustCompile(`^\[.+\]\s+failures:`).FindString(lines[0])
	if len(startMatchedStr) != 0 {
		for i := range lines {
			endMatchedStr := regexp.MustCompile(`\[.+\] test result: (\S+)\. (\d+) passed; (\d+) failed; .*`).FindString(lines[i])
			if len(endMatchedStr) != 0 {
				break
			}
			caseMatchedStr := regexp.MustCompile(`^\[.+\]\s+([A-Za-z0-9:_]+)`).FindString(lines[i])
			if len(caseMatchedStr) != 0 && !strings.Contains(caseMatchedStr, "failures:") {
				failDetail := strings.TrimSpace(strings.Split(caseMatchedStr, "]")[1])
				res = append(res, failDetail)
			}
		}
	}
	matchedStr := regexp.MustCompile(`test .* panicked at`).FindString(lines[0])
	if len(matchedStr) != 0 {
		detail := strings.TrimSpace(strings.Split(strings.Split(matchedStr, "...")[0], "test ")[1])
		res = append(res, detail)
	}
	return res
}

func MatchAndParseSQLStmtTest(logLine string) (bool, string) {
	if !strings.Contains(logLine, "level=fatal msg=") {
		return false, ""
	}
	testStmt := strings.Split(logLine, "\\\"")[1]
	testSplit := strings.Split(logLine, "run test [")
	if len(testSplit) < 2 {
		return false, ""
	}
	testName := strings.Split(testSplit[1], "] err")[0]
	return true, "[" + testName + "]:" + testStmt
}
