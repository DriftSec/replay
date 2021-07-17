package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
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
type ParamSet map[string]string
type HeaderSet map[string]string
type Method string

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

func DumpResponse(resp *http.Response) {

	fmt.Println(resp.Proto, resp.Status)
	for k, v := range resp.Header {
		fmt.Println(k+":", strings.Join(v, "; "))
	}
	fmt.Println("\n")
	fmt.Println(GetBodyString(resp.Body))

}

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

	// Handle case with the full http url in path. In that case,
	// ignore any host header that we encounter and use the path as request URL
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

	// Set the request body
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return &RequestConfig{}, fmt.Errorf("could not read request body: %s", err)
	}
	conf.RawBody = string(b)

	// Remove newline (typically added by the editor) at the end of the file
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

	// ! need to deal with other content types, i.e. JSON, XML
	if conf.ContentType == "application/xml" {
		ParseBodyXML(&conf)
	}

	if conf.ContentType == "application/json" {
		ParseBodyJSON(&conf)
	}

	if strings.HasPrefix(conf.ContentType, "multipart/form-data") {
		err := ParseBodyMultiPart(&conf)
		if err != nil {
			return &RequestConfig{}, err
		}

	}

	return &conf, nil
}

func ParseBodyXML(conf *RequestConfig) {

}

func ParseBodyJSON(conf *RequestConfig) {

}

func ParseBodyMultiPart(conf *RequestConfig) error {
	//multipart/form-data; boundary=------------------------948a6137eef50079
	tmp := strings.TrimSpace(conf.ContentType)
	parts := strings.Split(tmp, ";")
	Sections := []map[string]string{}
	if len(parts) != 2 {
		return errors.New("failed to parse form boundry")
	}
	boundry := strings.ReplaceAll(strings.Split(parts[1], "=")[1], "-", "")

	tmpBody := conf.RawBody

	end := regexp.MustCompile("[-]+" + boundry + "--$")
	tmpBody = end.ReplaceAllString(tmpBody, "")

	re := regexp.MustCompile("[-]+" + boundry + "\n")
	split := re.Split(tmpBody, -1)

	for _, i := range split {
		if len(i) == 0 {
			continue
		}
		Sections = append(Sections, mapBoundry(i))
	}
	conf.MultiPart = Sections
	return nil

}

func mapBoundry(section string) map[string]string {
	ret := make(map[string]string)
	// data := []string{}
	parts := strings.SplitN(section, "\n\n", 2)

	header := parts[0]
	body := parts[1]
	ret["body"] = body

	scanner := bufio.NewScanner(strings.NewReader(header))
	for scanner.Scan() {
		line := scanner.Text()
		pv := strings.SplitN(line, ":", 2)
		if strings.Contains(pv[1], "; ") {
			tmp := strings.Split(pv[1], "; ")
			ret[pv[0]] = tmp[0]
			for _, f := range tmp[1:] {
				fpv := strings.SplitN(f, "=", 2)
				ret[strings.TrimLeft(fpv[0], " ")] = fpv[1]
			}
		} else {
			ret[strings.TrimLeft(pv[0], " ")] = pv[1]
		}

	}

	return ret
}
