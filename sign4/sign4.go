// Package signs http requests for Amazon Web Services (AWS) using Signature Version 4.
//
// See http://docs.aws.amazon.com/general/latest/gr/signature-version-4.html
//
// There are two ways to use the package.
//
// 1. Sign: This is the simplified API; just fill in the required parameters to Sign and get a signed  request.
//
// 2. Step-by-Step: If for some reason you need more fine-grained control, you can walk through each step of the signing process. Roughly speaking, this is:
//
//		A. get a CanonicalRequest
//		B. build the CredentialScope
//		C. get the StringToSign
//		D. sign the StringToSign (with SignStringToSign)
//		E. get the AuthHeaderValue
//		F. add the AuthHeaderValue to the request.Header
package sign4

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	FMT_YYYYMMDD  = "20060102"
	FMT_AMZN_DATE = "20060102T150405Z07:00"
)

// Signs a ReusableRequest, and returns a copy of the request as a http.Request for use by http.Client.
//
// If the ReusableRequest has either a "Date" or a "x-amz-date" header, that date will be used in the signing
// process. Otherwise, Sign() will add "x-amz-date" header with the value of the current time (in UTC).
func (req *ReusableRequest) Sign(accessKey, secretKey, regionName, serviceName string) (hreq *http.Request, err error) {

	var t time.Time
	// see if we can derive a time from the request
	if dt := req.Header.Get("Date"); dt != "" {
		t, err = time.Parse(time.RFC1123, dt)
		if err != nil {
			return
		}
	} else if dt := req.Header.Get("x-amz-date"); dt != "" {
		t, err = time.Parse(FMT_AMZN_DATE, dt)
		if err != nil {
			return
		}
	} else {
		//set our own date
		t = time.Now().UTC()
		req.Header.Set("x-amz-date", t.Format(FMT_AMZN_DATE))
	}

	buff := new(bytes.Buffer)

	err = req.Write(buff)
	if err != nil {
		return
	}

	cr, err := CanonicalRequest(buff.String())
	if err != nil {
		return
	}

	credentialScope := CredentialScope(t, regionName, serviceName)
	stringToSign := StringToSign(cr.CanonicalRequest, credentialScope, t)
	signature, err := SignStringToSign(stringToSign, secretKey)
	if err != nil {
		return
	}

	authHeader := AuthHeaderValue(signature, accessKey, credentialScope, cr)
	req.Header.Set("Authorization", authHeader)
	out := req.ToHttpRequest()

	return &out, nil
}

// Get the finalized value for the "Authorization" header. The signature parameter is the output from SignStringToSign
func AuthHeaderValue(signature, accessKey, credentialScope string, cr *CanonicalRequestT) string {
	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, cr.Headers, signature)
}

type CanonicalRequestT struct {
	CanonicalRequest string
	Headers          string // semicolon delimited list of the headers in the canonical request
}

// Build a CanonicalRequestT from a regular request string
//
// See http://docs.aws.amazon.com/general/latest/gr/sigv4-create-canonical-request.html
func CanonicalRequest(req string) (cr *CanonicalRequestT, err error) {

	lines := strings.Split(req, "\r\n")

	if len(lines) < 1 {
		return nil, errors.New("Not enough data in the request: " + req)
	}

	out := make([]string, 3, len(lines))

	line1parts := strings.Split(lines[0], " ")
	if len(line1parts) < 3 {
		return nil, errors.New("Not enough data in the first line of request: " + lines[0])
	}

	reqUrl, err := url.ParseRequestURI(line1parts[1])
	if err != nil {
		return
	}

	out[0] = strings.ToUpper(line1parts[0])
	out[1] = getRawPath(line1parts[1])
	out[2], err = orderAndEncodeUrlValues(reqUrl.Query())

	if err != nil {
		return
	}

	// work on the headers
	hmap, sortedKeys := crHeaderMap(lines)
	sgnHeaders := make([]string, 0, len(sortedKeys))
	for _, hkey := range sortedKeys {
		hval := hmap[hkey]
		out = append(out, hkey+":"+hval)
		sgnHeaders = append(sgnHeaders, hkey)
	}

	headersSigned := strings.Join(sgnHeaders, ";")
	out = append(out, "\n"+headersSigned)

	// work on body
	bbody := getBody(lines)

	hashStr, err := hashSha256Body(bbody)
	if err != nil {
		return
	}
	out = append(out, hashStr)

	cr = &CanonicalRequestT{strings.Join(out, "\n"), headersSigned}

	return
}

func getRawPath(rawUrl string) string {
	// We can't use the norman URL functionality, because we need the raw unencoded path for
	// the canonical request, and URL.Path encodes things for us.

	if rawUrl == "/" {
		return "/"
	}
	parts := strings.SplitN(rawUrl, "?", 2)
	urlPath := parts[0]

	cleaned := path.Clean(urlPath)
	// Clean doesn't add the trailing slash, so add back if in the original path
	if strings.HasSuffix(urlPath, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned = cleaned + "/"
	}
	return cleaned
}

func getBody(reqLines []string) (body []byte) {
	blankIdx := 0
	for i, line := range reqLines {
		if line == "" {
			blankIdx = i
			break
		}
	}

	if blankIdx > 0 && len(reqLines) > blankIdx+1 {
		body = []byte(strings.Join(reqLines[blankIdx+1:], "\r\n"))
	}
	return
}

func orderAndEncodeUrlValues(values url.Values) (string, error) {

	//fmt.Println("values:", values)
	keys := make([]string, len(values))
	out := make([]string, 0, len(values))
	i := 0
	for k, _ := range values {
		keys[i] = url.QueryEscape(k)
		i++
	}

	sort.Strings(keys)

	for _, k := range keys {
		original_k, err := url.QueryUnescape(k)
		if err != nil {
			return "", err
		}
		vals := values[original_k]
		sort.Strings(vals)
		for _, dupVal := range vals {
			out = append(out, fmt.Sprintf("%v=%v", k, url.QueryEscape(dupVal)))
		}
	}

	return strings.Join(out, "&"), nil
}

// make a Canonical Request map of the headers
func crHeaderMap(lines []string) (headers map[string]string, sortedKeys []string) {
	sortedKeys = make([]string, 0, len(lines))
	headers = make(map[string]string)

	//fmt.Printf("sortedKeys: %v, len: %v cap: %v\n", sortedKeys, len(sortedKeys), cap(sortedKeys))
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		splitline := strings.SplitN(line, ":", 2)
		if len(splitline) == 2 {
			label := strings.ToLower(splitline[0])
			value := trimAll(splitline[1])
			if current, ok := headers[label]; ok {
				headers[label] = current + "," + value
			} else {
				headers[label] = value
				sortedKeys = append(sortedKeys, label)
			}
		}
	}
	sort.Strings(sortedKeys)
	return headers, sortedKeys
}

// Return the Credential Scope. See http://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html
func CredentialScope(t time.Time, regionName, serviceName string) string {
	return fmt.Sprintf("%s/%s/%s/aws4_request", t.UTC().Format(FMT_YYYYMMDD), regionName, serviceName)
}

// Create a "String to Sign". See http://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html
func StringToSign(canonicalRequest, credentialScope string, t time.Time) string {
	hash := sha256.New()
	hash.Write([]byte(canonicalRequest))
	timeStr := t.UTC().Format(FMT_AMZN_DATE)
	return fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%x",
		timeStr, credentialScope, hash.Sum(nil))
}

// Create the AWS Signature Version 4. See http://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
func SignStringToSign(sts, secretKey string) (string, error) {

	lines := strings.Split(sts, "\n")
	if len(lines) != 4 {
		return "", fmt.Errorf("Expected 4 lines in sts:\n%v", sts)
	}
	parts := strings.Split(lines[2], "/")
	if len(parts) != 4 {
		return "", fmt.Errorf("Expected 4 elements in: %v", lines[2])
	}

	dateStamp, region, service := parts[0], parts[1], parts[2]

	sk, err := SigningKey(secretKey, dateStamp, region, service)
	if err != nil {
		return "", err
	}

	signed, err := signHMAC(sk, sts)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", signed), nil

}

// Generate a "signing key" to sign the "String To Sign". See http://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
func SigningKey(awsKey, dateStamp, regionName, serviceName string) ([]byte, error) {

	key := []byte("AWS4" + awsKey)
	var err error

	data := []string{dateStamp, regionName, serviceName, "aws4_request"}
	for _, d := range data {
		key, err = signHMAC(key, d)
		if err != nil {
			return nil, err
		}
		//fmt.Printf("key: %x\n", key)
	}

	return key, nil

}

func signHMAC(key []byte, data string) ([]byte, error) {
	hmac := hmac.New(sha256.New, []byte(key))
	_, err := hmac.Write([]byte(data))
	if err != nil {
		return nil, err
	}
	return hmac.Sum(nil), nil
}

func hashSha256Body(body []byte) (string, error) {
	hash := sha256.New()
	//fmt.Printf("body: %v\n", body)
	if body == nil {
		_, err := hash.Write([]byte(""))
		if err != nil {
			return "", err
		}
	} else {
		_, err := hash.Write(body)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// Trim spaces from a header value string, per the Amazon spec
func trimAll(val string) string {
	inQuote := false
	priorWhitespace := false

	var buffer bytes.Buffer

	val = strings.TrimSpace(val) // do this to ensure priorWhitespace is really false
	for _, c := range val {
		if !inQuote && unicode.IsSpace(c) {
			if !priorWhitespace {
				priorWhitespace = true
				buffer.WriteRune(' ') //space
			} // else drop the space
		} else if c == '"' {
			inQuote = !inQuote
			priorWhitespace = false
			buffer.WriteRune(c)
		} else {
			priorWhitespace = false
			buffer.WriteRune(c)
		}
	}
	return buffer.String()
}

type ReusableBody struct {
	*bytes.Reader
}

// This is a noop function used to satisfy the io.ReadCloser interface.
func (b ReusableBody) Close() error {
	return nil // noop
}

// Request where the Body can be reused and reset. This type wraps http.Request.
// This type is used in the signing process because we need to read the request Body,
// and an http.Request is normally only avaliable to read once, making it unusable
// when we need it for the real request.
//
// If you are going to add or substitute the Body outside of the New* functions, use a ReusableBody and
// set the Content-Length of the request.
type ReusableRequest struct {
	*http.Request
}

// Create a new ReusableRequest.
//
// Warning: will read the body, possibly consuming it.
func NewReusableRequest(method, urlString string, body io.Reader) (*ReusableRequest, error) {
	req, err := http.NewRequest(method, urlString, nil)
	if err != nil {
		return nil, err
	}

	if body != nil {
		rb, err := makeReusableBody(body)
		if err != nil {
			return nil, err
		}
		req.Body = rb
		req.ContentLength = int64(rb.Len())
	}
	return &ReusableRequest{req}, nil
}

// Create a new ReusableRequest using a http.Reqeust.
//
// Warning: will read (and replace) the req.Body if it exists
func NewReusableRequestFromRequest(req *http.Request) (*ReusableRequest, error) {

	rreq := &ReusableRequest{req}
	if req.Body != nil {
		rb, err := makeReusableBody(req.Body)
		if err != nil {
			return nil, err
		}
		// copy the body
		b := make([]byte, rb.Len())
		_, err = rb.Read(b)
		rb.Seek(0, 0)
		if err != nil {
			return nil, err
		}
		rb2 := &ReusableBody{bytes.NewReader(b)}
		rreq.Body = rb
		req.Body = rb2
	}
	return rreq, nil
}

// Convert a ReusableRequest to a http.Request
func (req *ReusableRequest) ToHttpRequest() (hreq http.Request) {
	hreq.Method = req.Method
	hreq.URL = req.URL
	hreq.Proto = req.Proto
	hreq.ProtoMajor = req.ProtoMajor
	hreq.ProtoMinor = req.ProtoMinor
	hreq.Header = req.Header
	hreq.Body = req.Body
	hreq.ContentLength = req.ContentLength
	hreq.TransferEncoding = req.TransferEncoding
	hreq.Close = req.Close
	hreq.Host = req.Host
	hreq.Form = req.Form
	hreq.PostForm = req.PostForm
	hreq.MultipartForm = req.MultipartForm
	hreq.Trailer = req.Trailer
	hreq.RemoteAddr = req.RemoteAddr
	hreq.RequestURI = req.RequestURI
	hreq.TLS = req.TLS
	return
}

func makeReusableBody(body io.Reader) (rb *ReusableBody, err error) {

	//fmt.Printf("Type of body: %T\n", body)
	if body != nil {
		switch v := body.(type) {
		case *bytes.Reader:
			rb = &ReusableBody{v}
		case *ReusableBody:
			rb = v
		default:
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(body)
			if err != nil {
				return nil, err
			}
			reader := bytes.NewReader(buf.Bytes())
			rb = &ReusableBody{reader}
		}
	} else {
		rb = nil
	}
	return rb, nil
}

func (req *ReusableRequest) Write(w io.Writer) error {
	return req.write(w, false)
}

func (req *ReusableRequest) WriteProxy(w io.Writer) error {
	return req.write(w, true)
}

func (req *ReusableRequest) write(w io.Writer, usingProxy bool) error {
	rb, ok := req.Body.(*ReusableBody)
	if !ok && req.Body != nil {
		return errors.New("Not sure body can be reused (did req.Body get changed?)")
	}
	var err error
	if req.Body != nil {
		defer rb.Seek(0, 0)
	}
	if !usingProxy {
		err = req.Request.Write(w)
	} else {
		err = req.Request.WriteProxy(w)
	}

	if err != nil {
		return err
	}

	return nil
}
