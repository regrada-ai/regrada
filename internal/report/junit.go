package report

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

type testSuite struct {
	XMLName  xml.Name   `xml:"testsuite"`
	Tests    int        `xml:"tests,attr"`
	Failures int        `xml:"failures,attr"`
	Cases    []testCase `xml:"testcase"`
}

type testCase struct {
	Name    string    `xml:"name,attr"`
	Failure *testFail `xml:"failure,omitempty"`
}

type testFail struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func WriteJUnit(summary RunSummary, path string) error {
	suite := testSuite{Tests: len(summary.Cases)}
	for _, c := range summary.Cases {
		failures := errorViolations(c)
		if len(failures) > 0 {
			suite.Failures++
			suite.Cases = append(suite.Cases, testCase{
				Name: c.CaseID,
				Failure: &testFail{
					Message: "policy violations",
					Body:    strings.Join(failures, "\n"),
				},
			})
			continue
		}
		suite.Cases = append(suite.Cases, testCase{Name: c.CaseID})
	}

	data, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}
	data = append([]byte(xml.Header), data...)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func errorViolations(c CaseSummary) []string {
	var out []string
	for _, v := range c.Violations {
		if v.Severity == "error" {
			out = append(out, v.PolicyID+": "+v.Message)
		}
	}
	return out
}
