sign4
=====

Package signs http requests for Amazon Web Services (AWS) using Signature Version 4.

See http://docs.aws.amazon.com/general/latest/gr/signature-version-4.html

There are two ways to use the package:

1. `Sign`: This is the simplified API; just fill in the required parameters to `Sign` 
   and get a signed request.
2. Step-by-Step: If for some reason you need more fine-grained control, you can walk 
   through each step of the signing process. Roughly speaking, this is:
    1. get a `CanonicalRequest`
    2. build the `CredentialScope`
    3. get the `StringToSign`
    4. sign the String to Sign (with `SignStringToSign`)
    5. get the `AuthHeaderValue`
    6. add the AuthHeaderValue to the request.Header
    

Example of `Sign`
-----------------

```Go
func ExampleReusableRequest_Sign() {
	body := strings.NewReader("Action=ListUsers&Version=2010-05-08")

	request, err := sign4.NewReusableRequest("POST", "http://service.example.com", body)
	if err != nil {
		// do something with the error
	}

	// We will set the time manually (so the test works), but if you don't do this, Sign()
	// will set the X-Amz-Date header with the current time.
	t := time.Date(2013, time.October, 31, 10, 30, 0, 0, time.UTC)
	request.Header.Set("x-amz-date", t.Format(sign4.FMT_AMZN_DATE))

	//insert your logic for getting credentials here
	accessKey, secretKey := getCredentials()

	httpRequest, err := request.Sign(accessKey, secretKey, "us-east-1", "service")
	if err != nil {
		// do something with the error
	}

	// You can now use the httpRequest normally (e.g. submit to http.Client.Do)
	// resp, err := http.DefaultClient.Do(httpRequest)

	// Here, we're just going to output the request so we can see the signature.
	buff := new(bytes.Buffer)
	httpRequest.Write(buff)
	fmt.Println(buff.String())

	// Output:
	// POST / HTTP/1.1
	// Host: service.example.com
	// User-Agent: Go 1.1 package http
	// Content-Length: 35
	// Authorization: AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20131031/us-east-1/service/aws4_request, SignedHeaders=content-length;host;user-agent;x-amz-date, Signature=9a0659143c33772a5293374b60b6ade850d8f7c82bdeb657917c7fd3cba86e4d
	// X-Amz-Date: 20131031T103000Z

	// Action=ListUsers&Version=2010-05-08
}
```

Testing
-------
There is a subdirectory with a handful of Amazon-defined test cases. To run these tests, do:

`go test -test-suite-dir /PATH/TO/Tests`