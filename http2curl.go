package http2curl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// CurlCommand contains exec.Command compatible slice + helpers
type CurlCommand []string

// append appends a string to the CurlCommand
func (c *CurlCommand) append(newSlice ...string) {
	*c = append(*c, newSlice...)
}

// String returns a ready to copy/paste command
func (c *CurlCommand) String() string {
	return strings.Join(*c, " ")
}

func bashEscape(str string) string {
	return `'` + strings.Replace(str, `'`, `'\''`, -1) + `'`
}

// GetCurlCommand returns a CurlCommand corresponding to an http.Request
func GetCurlCommand(req *http.Request) (*CurlCommand, error) {
	if req.URL == nil {
		return nil, fmt.Errorf("getCurlCommand: invalid request, req.URL is nil")
	}

	command := CurlCommand{}

	command.append("curl")

	schema := req.URL.Scheme
	requestURL := req.URL.String()
	if schema == "" {
		schema = "http"
		if req.TLS != nil {
			schema = "https"
		}
		requestURL = schema + "://" + req.Host + req.URL.Path
	}

	if schema == "https" {
		command.append("-k")
	}

	command.append("-X", bashEscape(req.Method))
	var bodyByte []byte
	if req.GetBody != nil {
		if bodyIO, err := req.GetBody(); err == nil {
			defer bodyIO.Close()
			bodyByte, err = io.ReadAll(bodyIO)
			if err != nil {
				return nil, fmt.Errorf("getCurlCommand:  read from req.GetBody() error: %w", err)
			}
		}
	}

	if len(bodyByte) == 0 && req.Body != nil {
		var err error
		bodyByte, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("getCurlCommand: buffer read from body error: %w", err)
		}
		// reset body for potential re-reads
		req.Body = io.NopCloser(bytes.NewBuffer(bodyByte))
	}

	if len(bodyByte) > 0 {
		bodyEscaped := bashEscape(string(bodyByte))
		command.append("-d", bodyEscaped)
	}

	var keys []string

	for k := range req.Header {
		if strings.EqualFold(k, "Content-Length") { //drop centent-length header, it will be changed by modifing parameters
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		command.append("-H", bashEscape(fmt.Sprintf("%s: %s", k, strings.Join(req.Header[k], " "))))
	}

	command.append(bashEscape(requestURL))

	command.append("--compressed")

	return &command, nil
}
