package sign4_test

import (
	"fmt"
	. "launchpad.net/gocheck"
	"testing"

	"bufio"
	"bytes"
	"errors"
	"flag"
	"github.com/p-lewis/awsgolang/sign4"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var testSuiteDir = flag.String("test-suite-dir", "", "Directory containing the aws4_testsute files.")

func Test(t *testing.T) { TestingT(t) }

type Sign4Suite struct {
	request1 *sign4.ReusableRequest
	request2 *sign4.ReusableRequest
}

var _ = Suite(&Sign4Suite{})

func (s *Sign4Suite) SetUpTest(c *C) {
	req, err := sign4.NewReusableRequest("GET", "http://host.foo.com/?foo=Zoo&foo=aha",
		bytes.NewReader([]byte("Hello world")))
	c.Assert(err, IsNil)
	req.Header.Set("User-Agent", "Dummy Agent")
	req.Header.Set("Date", "date goes here")
	req.Header.Set("My-header1", "a   b   c")
	req.Header.Set("My-header2", "   \"a    b   c\"  ")
	s.request1 = req

	req, err = sign4.NewReusableRequest("GET", "http://host.foo.com/?foo=Zoo&foo=aha", nil)
	c.Assert(err, IsNil)
	req.Header.Set("User-Agent", "")
	req.Header.Set("Date", "Mon, 09 Sep 2011 23:36:00 GMT")
	s.request2 = req
}

func (s *Sign4Suite) TestReusableRequest(c *C) {

	req := s.request1

	buf := new(bytes.Buffer)
	err := req.Write(buf)
	c.Assert(err, IsNil)
	s1 := buf.String()
	//fmt.Println(buf.String())

	buf.Reset()
	err = req.Write(buf)
	c.Assert(err, IsNil)
	s2 := buf.String()

	c.Assert(s1, Equals, s2)

}

func (s *Sign4Suite) TestSign(c *C) {
	req := s.request2
	hreq, err := req.Sign("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "us-east-1", "host")
	c.Assert(err, IsNil)

	expect := "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, " +
		"SignedHeaders=date;host, Signature=be7148d34ebccdc6423b19085378aa0bee970bdc61d144bd1a8c48c33079ab09"
	c.Assert(hreq.Header.Get("Authorization"), Equals, expect)
	c.Assert(req.Header.Get("Authorization"), Equals, expect)
}

func (s *Sign4Suite) TestCanonicalRequest(c *C) {

	expect := "GET\n/\nfoo=Zoo&foo=aha\ndate:Mon, 09 Sep 2011 23:36:00 GMT\nhost:host.foo.com\n\n" +
		"date;host\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	buf := new(bytes.Buffer)
	err := s.request2.Write(buf)
	c.Assert(err, IsNil)

	cr, err := sign4.CanonicalRequest(buf.String())

	c.Assert(err, IsNil)
	c.Assert(cr.Headers, Equals, "date;host")
	//fmt.Println(cr)
	c.Assert(cr.CanonicalRequest, Equals, expect)
}

func (s *Sign4Suite) TestStringToSign(c *C) {

	buf := new(bytes.Buffer)
	err := s.request2.Write(buf)
	c.Assert(err, IsNil)

	cr, err := sign4.CanonicalRequest(buf.String())

	c.Assert(err, IsNil)
	t := time.Date(2011, time.September, 9, 23, 36, 0, 0, time.UTC)
	sts := sign4.StringToSign(cr.CanonicalRequest, "20110909/us-east-1/host/aws4_request", t)

	expect := "AWS4-HMAC-SHA256\n20110909T233600Z\n20110909/us-east-1/host/aws4_request\ne25f777ba161a0f1baf778a87faf057187cf5987f17953320e3ca399feb5f00d"

	c.Assert(sts, Equals, expect)

	//fmt.Println(sts)
}

func (s *Sign4Suite) TestSignStringToSign(c *C) {
	sts := "AWS4-HMAC-SHA256\n20110909T233600Z\n20110909/us-east-1/iam/aws4_request\n3511de7e95d28ecd39e9513b642aee07e54f4941150d8df8bf94b328ef7e55e2"
	ssts, err := sign4.SignStringToSign(sts, "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")
	c.Assert(err, IsNil)
	//fmt.Println("ssts:", ssts)
	c.Assert(ssts, Equals, "ced6826de92d2bdeed8f846f0bf508e8559e98e4b0199114b84c54174deb456c")

}

func (s *Sign4Suite) TestSigningKey(c *C) {
	key := "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	dateStamp := "20120215"
	regionName := "us-east-1"
	serviceName := "iam"

	k, err := sign4.SigningKey(key, dateStamp, regionName, serviceName)
	c.Assert(err, IsNil)
	c.Assert(fmt.Sprintf("%x", k), Equals, "f4780e2d9f65fa895f9c67b32ce1baf0b0d8a43505a000a1a9e090d414db404d")
}

func (s *Sign4Suite) TestCredentialScope(c *C) {
	t := time.Date(2011, time.September, 9, 23, 36, 0, 0, time.UTC)
	scope := sign4.CredentialScope(t, "us-east-1", "iam")
	c.Assert(scope, Equals, "20110909/us-east-1/iam/aws4_request")
}

func (s *Sign4Suite) TestSignInsertsTime(c *C) {
	t := time.Now()

	//time.Sleep(1 * time.Second)  // The header request only has second precision, so we need to wait at least a second

	req := s.request2
	req.Header.Del("Date")
	val := req.Header.Get("Date")
	c.Assert(val, Equals, "")
	val = req.Header.Get("x-amz-date")
	c.Assert(val, Equals, "")
	_, err := req.Sign("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "us-east-1", "host")
	c.Assert(err, IsNil)
	val = req.Header.Get("x-amz-date")
	c.Assert(val, Not(Equals), "")
	t2, err := time.Parse(sign4.FMT_AMZN_DATE, val)
	c.Assert(err, IsNil)
	elapsed := t2.Sub(t)
	c.Assert(elapsed < time.Second, Equals, true)

}

func (s *Sign4Suite) TestAWSSuite(c *C) {
	if *testSuiteDir == "" {
		c.Skip("-test-suite-dir not provided, skipping aws4 testsuite")
	}

	accessKey := "AKIDEXAMPLE"
	secretKey := "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	regionName := "us-east-1"
	serviceName := "host"

	tests := []string{"get-header-value-trim", "get-vanilla-query", "get-relative",
		"get-relative-relative", "get-slash", "get-slash-dot-slash",
		"get-slashes", "get-slash-pointless-dot", "get-space", "get-unreserved",
		"get-utf8", "get-vanilla", "get-vanilla-empty-query-key", "get-vanilla-query",
		"get-vanilla-query-order-key", "get-vanilla-query-order-key-case",
		"get-vanilla-query-order-value", "get-vanilla-query-unreserved",
		"get-vanilla-ut8-query", "post-header-key-case", "post-header-key-sort",
		"post-header-value-case", "post-vanilla", "post-vanilla-empty-query-value",
		"post-vanilla-query",
		//"post-vanilla-query-nonunreserved" // this one is pretty pathological, FIXME ?
		//"post-vanilla-query-space"		// don't think this a valid http request (a space in the URI?)
		"post-x-www-form-urlencoded", "post-x-www-form-urlencoded-parameters",
	}
	// broken tests: "get-header-key-duplicate", "get-header-value-order"
	// see https://forums.aws.amazon.com/thread.jspa?messageID=491017

	//buff := new(bytes.Buffer)

	for _, test := range tests {
		c.Log("TestAWSSuite test: %v", test)
		reqFileName := filepath.Join(*testSuiteDir, test+".req")
		creqFileName := filepath.Join(*testSuiteDir, test+".creq")
		stsFileName := filepath.Join(*testSuiteDir, test+".sts")
		sreqFileName := filepath.Join(*testSuiteDir, test+".sreq")

		readBytes, err := ioutil.ReadFile(reqFileName)
		c.Assert(err, IsNil)
		//fmt.Println("readBytes:\n", readBytes)

		// canonical request
		canonReq, err := sign4.CanonicalRequest(string(readBytes))
		c.Assert(err, IsNil)
		readBytes, err = ioutil.ReadFile(creqFileName)
		c.Assert(err, IsNil)
		c.Assert(canonReq.CanonicalRequest, Equals, string(readBytes))

		// string to sign
		t, err := getTimeFromCR(canonReq)
		c.Assert(err, IsNil)
		credentialScope := sign4.CredentialScope(*t, regionName, serviceName)
		stringToSign := sign4.StringToSign(canonReq.CanonicalRequest, credentialScope, *t)

		readBytes, err = ioutil.ReadFile(stsFileName)
		c.Assert(err, IsNil)
		c.Assert(stringToSign, Equals, string(readBytes))

		// signed
		signature, err := sign4.SignStringToSign(stringToSign, secretKey)
		c.Assert(err, IsNil)
		authHdrVal := sign4.AuthHeaderValue(signature, accessKey, credentialScope, canonReq)

		// Authorized value
		sreq, err := getAWSSuiteReq(sreqFileName)
		c.Assert(err, IsNil)
		c.Assert(authHdrVal, Not(Equals), "")
		c.Assert(authHdrVal, Equals, sreq.Header.Get("Authorization"))
	}
}

func getAWSSuiteReq(reqFileName string) (*sign4.ReusableRequest, error) {
	readBytes, err := ioutil.ReadFile(reqFileName)
	//fmt.Printf("Read: \n%v\n", string(readBytes))
	if err != nil {
		return nil, err
	}

	// fix lowercase http, which causes issues in the parser
	if reqStr := string(readBytes); strings.Contains(reqStr, "http/1.1") {
		readBytes = []byte(strings.Replace(reqStr, "http/1.1", "HTTP/1.1", 1))
	}

	buff := new(bytes.Buffer)
	_, err = buff.Write(readBytes)
	if err != nil {
		return nil, err
	}

	hreq, err := http.ReadRequest(bufio.NewReader(buff))
	if err != nil {
		return nil, err
	}

	req, err := sign4.NewReusableRequestFromRequest(hreq)
	if err != nil {
		return nil, err
	}

	// add a blank user agent if one doesn't exist
	_, ok := req.Header["User-Agent"]
	if !ok {
		req.Header.Set("User-Agent", "")
	}
	return req, nil
}

func getTimeFromCR(req *sign4.CanonicalRequestT) (t *time.Time, err error) {

	lines := strings.Split(req.CanonicalRequest, "\n")

	for _, line := range lines {
		if len(line) > 5 && line[:5] == "date:" {
			tm, err := time.Parse(time.RFC1123, line[5:])
			return &tm, err
		}
	}

	return nil, errors.New("Couldn't find a date. (Sob).")
}

