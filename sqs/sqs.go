package sqs

import (
	//"bytes"
	"encoding/xml"
	"fmt"
	"github.com/p-lewis/awsgolang/auth"
	"github.com/p-lewis/awsgolang/sign4"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	AWS_API_VERSION = "2012-11-05"
	SERVICE_NAME    = "sqs"
)

// The SQS type encapsulates operations with an SQS region.
type SQS struct {
	Credentials   *auth.Credentials
	Region        *Region
	ClientFactory func() *http.Client // Factory function that builds an http.Client for requests
}

// The queue type encapsulates operations with an SQS Queue.
type Queue struct {
	*SQS
	Name string
	Url  string
}

type Region struct {
	Name     string // the canonical name of this region.
	Endpoint string // URL for the endpoint of this region
}

// Create a new SQS structure.
//
// If factoryFn is nil, the default client is inserted.
// func New(cred *auth.Credentials, region *Region, factoryFn func() *http.Client) {
// 	sqs := SQS{cred, region, factoryFn}
// 	if factoryFn == nil {
// 		sqs.ClientFactory = func()(*http.Client) { return http.DefaultClient }
// 	}
// 	return
// }

func DefaultClientFactory() *http.Client {
	return http.DefaultClient
}

func (sqs *SQS) CreateQueue(name string) (sqsQueue *Queue, response *CreateQueueResponse, err error) {

	vals := sqs.defaultValues("CreateQueue")
	vals.Set("QueueName", name)

	url := fmt.Sprintf("%v/?%v", sqs.Region.Endpoint, vals.Encode())
	req, err := sign4.NewReusableRequest("GET", url, nil)
	if err != nil {
		return
	}

	respBody, err := sqs.makeRequest(req)
	if err != nil {
		return
	}

	response = &CreateQueueResponse{}
	errResponse := &ErrorResponse{}
	err = unmarshalResponse(respBody, response, errResponse)
	if err != nil {
		return nil, nil, err
	}

	sqsQueue = &Queue{SQS: sqs, Name: name, Url: response.QueueUrl}

	return
}

func (q *Queue) DeleteQueue() (*DeleteQueueResponse, error) {
	vals := q.SQS.defaultValues("DeleteQueue")
	url := fmt.Sprintf("%v/?%v", q.Url, vals.Encode())
	//fmt.Println("url:", q.Url)
	req, err := sign4.NewReusableRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	respBody, err := q.SQS.makeRequest(req)
	if err != nil {
		return nil, err
	}

	response, errResponse := &DeleteQueueResponse{}, &ErrorResponse{}
	err = unmarshalResponse(respBody, response, errResponse)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (sqs *SQS) defaultValues(action string) (vals *url.Values) {
	vals = &url.Values{}
	vals.Set("Action", action)
	vals.Set("AWSAccessKeyId", sqs.Credentials.AccessKey)
	vals.Set("Version", AWS_API_VERSION)
	return
}

func (sqs *SQS) makeRequest(rreq *sign4.ReusableRequest) (respBody []byte, err error) {
	cred := sqs.Credentials
	hreq, err := rreq.Sign(cred.AccessKey, cred.SecretKey, sqs.Region.Name, SERVICE_NAME)
	if err != nil {
		return
	}

	client := sqs.ClientFactory()
	resp, err := client.Do(hreq)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	respBody, err = ioutil.ReadAll(resp.Body)
	return
}

type BodyUnmarshaller interface {
	SetRawResponse(rawResponse []byte)
}

type BodyUnmarshallerError interface {
	BodyUnmarshaller
	error
}

// Try to convert a response to a "good" type.
// Fall back the knownError type.
// Fall back to a generic error if neither of those work
func unmarshalResponse(body []byte, goodResponse BodyUnmarshaller, knownErrResponse BodyUnmarshallerError) (err error) {
	// check first if we have a successful conversion
	err = xml.Unmarshal(body, goodResponse)
	if err == nil {
		goodResponse.SetRawResponse(body)
		return
	}

	err = xml.Unmarshal(body, knownErrResponse)
	if err == nil {
		knownErrResponse.SetRawResponse(body)
		return knownErrResponse
	}

	return fmt.Errorf("sqs.unmarshalResponse: Unable to unmarshal body data to either %T or %T, body: %s",
		goodResponse, knownErrResponse, body)
}

type CreateQueueResponse struct {
	XMLName     xml.Name `xml:"http://queue.amazonaws.com/doc/2012-11-05/ CreateQueueResponse"`
	QueueUrl    string   `xml:"CreateQueueResult>QueueUrl"`
	RequestId   string   `xml:"ResponseMetadata>RequestId"`
	RawResponse []byte   // contains the raw xml data in the response
}

func (cqr *CreateQueueResponse) SetRawResponse(rawResponse []byte) {
	cqr.RawResponse = rawResponse
}

type DeleteQueueResponse struct {
	XMLName     xml.Name `xml:"http://queue.amazonaws.com/doc/2012-11-05/ DeleteQueueResponse"`
	RequestId   string   `xml:"ResponseMetadata>RequestId"`
	RawResponse []byte
}

func (dqr *DeleteQueueResponse) SetRawResponse(rawResponse []byte) {
	dqr.RawResponse = rawResponse
}

type ErrorResponse struct {
	XMLName     xml.Name  `xml:"http://queue.amazonaws.com/doc/2012-11-05/ ErrorResponse"`
	ErrorInfo   ErrorInfo `xml:"Error"`
	RequestId   string
	RawResponse []byte
}

type ErrorInfo struct {
	Type, Code, Message, Detail string
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("sqs.ErrorResponse Type: %v, Code: %v Message: %v",
		e.ErrorInfo.Type, e.ErrorInfo.Code, e.ErrorInfo.Message)
}

func (e *ErrorResponse) SetRawResponse(rawResponse []byte) {
	e.RawResponse = rawResponse
}
