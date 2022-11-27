package main

import (
	"bytes"
	"path/filepath"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"errors"
)

const testDataDir = "testdata"
var testTerms = []string{"linux mint", "ubuntu 10.04", "xubuntu"}

type FakeStdout struct {
	output []byte
}

func (o *FakeStdout) Write(p []byte) (int, error) {
	o.output = append(o.output, p...)

	return 1, nil
}

type FakeHTTPClient struct{}

func getTestFileNameForURL(u *url.URL) (string, error) {
	urlPath := u.Path

	for _, term := range testTerms {
		if strings.Contains(urlPath, term) {
			return strings.Replace(term, " ", "_", -1) + ".txt", nil
		}
	}

	return "", errors.New("Failed to find a file for URL " + urlPath)
}

func getTestResponseBodyForURL(u *url.URL) ([]byte, error) {
	testFilePath, err := getTestFileNameForURL(u)

	if err != nil {
		return nil, err
	}

	return getFileContents(testFilePath)
}

func (*FakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	responseBody, err := getTestResponseBodyForURL(req.URL)

	if err != nil {
		return nil, err
	}

	reader := ioutil.NopCloser(bytes.NewReader(responseBody))
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body: reader,
	}

	return response, nil
}

func TestRun(t *testing.T) {
	httpClient := &FakeHTTPClient{}
	out := &FakeStdout{}

	os.Args = append(os.Args, "-terms")
	os.Args = append(os.Args, strings.Join(testTerms, ","))
	os.Args = append(os.Args, "-number")
	os.Args = append(os.Args, "5")
	os.Args = append(os.Args, "-suffix")
	os.Args = append(os.Args, "i386")
	run(httpClient, out)

	actualOutput := out.output
	expectedOutput, err := getFileContents("output.txt")

	if err != nil {
		t.Errorf("Error getting expected output from test file: %s", err)
	}

	if !bytes.Equal(actualOutput, expectedOutput) {
		t.Errorf(
			"Actual output did not match expected output.\nACTUAL\n%s\nEXPECTED\n%s",
			actualOutput,
			&expectedOutput,
		)
	}
}

func getFileContents(fileName string) ([]byte, error) {
	c, err := os.ReadFile(filepath.Join(testDataDir, fileName))

	if err != nil {
		return nil, err
	}
	return c, nil
}
