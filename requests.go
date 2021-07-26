package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
)

type RequestConfig struct {
	Method      string
	ContentType string
	Headers     map[string]string
	Query       map[string]string
	Params      map[string]string
	MultiPart   []map[string]string
	Url         string
	RawBody     string
	Scheme      string
}

var (
	Client *http.Client
)

func init() {
	cfg := &tls.Config{
		InsecureSkipVerify: true,
	}

	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig: cfg,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	cookieJar, _ := cookiejar.New(nil)
	Client = &http.Client{Transport: tr,
		Jar: cookieJar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}

}

// CreateReq parses a *RequestConfig and returns a *http.Request ready for use in DoRequest, or error
func CreateReq(reqConf *RequestConfig) (*http.Request, error) {
	u, _ := url.ParseRequestURI(reqConf.Url)
	urlStr := u.String()
	Body := bytes.NewBuffer([]byte(reqConf.RawBody))
	r, _ := http.NewRequest(reqConf.Method, urlStr, Body) // URL-encoded payload

	q := r.URL.Query()

	for k := range reqConf.Params {
		q.Add(k, reqConf.Params[k])
	}

	r.URL.RawQuery = q.Encode()
	for k, v := range reqConf.Headers {
		r.Header.Add(k, v)
	}
	r.Header.Add("Content-Type", reqConf.ContentType)

	return r, nil
}

// DoRequest takes a *http.request and makes the request. returns an http.response or an error.  proxy string is optional
func DoRequest(r *http.Request, proxy string) (*http.Response, error) {
	if proxy != "" {
		proxyUrl, _ := url.Parse(proxy)
		Client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, Proxy: http.ProxyURL(proxyUrl)}
	}

	resp, err := Client.Do(r)
	if err != nil {
		return nil, err
	}
	// defer resp.Body.Close()
	return resp, nil
}

// GetBodyString is a wrapper arround io.reader that will return a string from an http.request or http.response body, and replaces body content for use elsewhere.
func GetReqBody(r *http.Request) io.ReadCloser   { return r.Body }
func GetRespBody(r *http.Response) io.ReadCloser { return r.Body }
func GetBodyString(rc io.ReadCloser) string {
	// defer rc.Close()
	bodyBytes, err := ioutil.ReadAll(rc)
	if err != nil {
		fmt.Println("[ERROR] failed to read response body")
		return ""
	}
	rc = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	return string(bodyBytes)
}

// DumpRawRequest dumps a burp style request to a file.
func DumpRawRequest(req *http.Request, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	w := bufio.NewWriter(f)

	qp := ""
	if len(req.URL.RawQuery) > 0 {
		qp = "?" + req.URL.RawQuery
	}
	w.WriteString(req.Method + " " + req.URL.Path + qp + " " + req.Proto + "\n")
	w.WriteString("Host: " + req.Host + "\n")
	for k := range req.Header {
		w.WriteString(k + ": " + req.Header.Get(k) + "\n")
	}

	if req.Method != "GET" {
		w.WriteString("\n\n")
		w.WriteString(GetBodyString(req.Body))
	}
	w.WriteString("\n\n")
	w.Flush()
}

// PrintResponse prints the full http response to stdout
func PrintResponse(resp *http.Response) {
	fmt.Println(resp.Proto, resp.Status)
	for k, v := range resp.Header {
		fmt.Println(k+":", strings.Join(v, "; "))
	}
	fmt.Printf("\n\n")
	fmt.Println(GetBodyString(resp.Body))

}

// ReadRawRequest parses a burp style request file and returns a *RequestConfig for use in CreateRequest
func ReadRawRequest(path string, scheme string) (*RequestConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return &RequestConfig{}, fmt.Errorf("could not open request file: %s", err)
	}
	defer file.Close()

	r := bufio.NewReader(file)

	s, err := r.ReadString('\n')

	if err != nil {
		return &RequestConfig{}, fmt.Errorf("could not read request: %s", err)
	}

	parts := strings.Split(s, " ")
	if len(parts) < 3 {
		return &RequestConfig{}, fmt.Errorf("malformed request supplied")
	}

	var conf RequestConfig
	conf.Scheme = scheme
	conf.Headers = make(map[string]string)
	conf.Params = make(map[string]string)
	conf.Query = make(map[string]string)

	// Set the request Method
	conf.Method = parts[0]

	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)

		if err != nil || line == "" {
			break
		}

		p := strings.SplitN(line, ":", 2)
		if len(p) != 2 {
			continue
		}

		if strings.EqualFold(p[0], "content-length") {
			continue
		}
		if strings.EqualFold(p[0], "content-type") {
			conf.ContentType = strings.TrimSpace(p[1])
		}

		conf.Headers[strings.TrimSpace(p[0])] = strings.TrimSpace(p[1])
	}

	var tmpUrl string
	if strings.HasPrefix(parts[1], "http") {
		parsed, err := url.Parse(parts[1])
		if err != nil {
			return &RequestConfig{}, fmt.Errorf("could not parse request URL: %s", err)
		}

		tmpUrl = parts[1]
		conf.Headers["Host"] = parsed.Host
	} else {
		tmpUrl = conf.Scheme + "://" + conf.Headers["Host"] + parts[1]
	}

	uq := strings.SplitN(tmpUrl, "?", 2)
	conf.Url = uq[0]
	if len(uq) >= 2 {
		for _, set := range strings.Split(uq[1], "&") {
			pv := strings.Split(set, "=")
			conf.Query[pv[0]] = pv[1]
		}
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return &RequestConfig{}, fmt.Errorf("could not read request body: %s", err)
	}
	conf.RawBody = string(b)

	if strings.HasSuffix(conf.RawBody, "\r\n") {
		conf.RawBody = conf.RawBody[:len(conf.RawBody)-2]
	} else if strings.HasSuffix(conf.RawBody, "\n") {
		conf.RawBody = conf.RawBody[:len(conf.RawBody)-1]
	}

	if conf.ContentType == "application/x-www-form-urlencoded" {
		for _, set := range strings.Split(conf.RawBody, "&") {
			pv := strings.Split(set, "=")
			if len(pv) < 2 {
				continue
			}
			conf.Params[pv[0]] = pv[1]
		}
	}

	return &conf, nil
}

// ReplaceVars handles parsing a *RequestConfig and replaces any variables in the repl map. returns a modified *RequestConfig
func (rc *RequestConfig) ReplaceVars(repl map[string]string) *RequestConfig {
	for varname, value := range repl {
		for hk, hv := range rc.Headers {
			rc.Headers[hk] = strings.ReplaceAll(hv, "{{"+varname+"}}", value)
		}
		for qk, qv := range rc.Query {
			rc.Query[qk] = strings.ReplaceAll(qv, "{{"+varname+"}}", value)
		}
		for pk, pv := range rc.Query {
			rc.Params[pk] = strings.ReplaceAll(pv, "{{"+varname+"}}", value)
		}
		rc.Url = strings.ReplaceAll(rc.Url, "{{"+varname+"}}", value)
		rc.RawBody = strings.ReplaceAll(rc.RawBody, "{{"+varname+"}}", value)

	}
	return rc
}
