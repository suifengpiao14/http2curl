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
	return `'` + strings.ReplaceAll(str, `'`, `'\''`) + `'`
}

var (
	// 使用排除部分头部是为了简化curl命令,能保证用户自定义的头部一定存在，保证curl命令能用，若采用容许头部，则用户自定义头部一般会被忽略，导致curl命令不能用。
	IgnoredHeaders = map[string]struct{}{
		"Accept-Encoding":   {},
		"User-Agent":        {},
		"Transfer-Encoding": {},
		"Connection":        {},
		"Expect":            {},
		"Content-Length":    {},
		"Referer":           {},
		"X-Real-Address":    {},
		"X-Real-Hos":        {},
		"Origin":            {},
		"Accept-Language":   {},
		"X-Real-Host":       {},
		"Accept":            {},
		"X-Forwarded-For":   {},
	}
	mustIgnoredHeaders = map[string]struct{}{ // 必须忽略的头，外部不能改，不可用动态修改全局变量IgnoredHeaders，会存在并发问题
		"Content-Length": {},
	}
)

// GetCurlCommand returns a CurlCommand corresponding to an http.Request
func GetCurlCommand(req *http.Request) (*CurlCommand, error) {
	if req == nil {
		return nil, fmt.Errorf("getCurlCommand: invalid request")
	}
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
		if _, ok := mustIgnoredHeaders[k]; ok {
			continue
		}
		if _, ok := IgnoredHeaders[k]; ok {
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
