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

func DefaultClientFactory() *http.Client {
	return http.DefaultClient
}

func (sqs *SQS) CreateQueue(name string) (sqsQueue *Queue, cqResponse *CreateQueueResponse, err error) {

	vals := sqs.defaultValues("CreateQueue")
	vals.Set("QueueName", name)

	url := fmt.Sprintf("%v/?%v", sqs.Region.Endpoint, vals.Encode())
	req, err := sign4.NewReusableRequest("GET", url, nil)
	if err != nil {
		return
	}

	httpResponse, err := sqs.makeRequest(req)
	if err != nil {
		return
	}

	cqResponse = &CreateQueueResponse{}
	errResponse := &ErrorResponse{}
	err = unmarshalResponse(httpResponse, cqResponse, errResponse)
	if err != nil {
		return nil, nil, err
	}

	sqsQueue = &Queue{SQS: sqs, Name: name, Url: cqResponse.QueueUrl}

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
	httpResponse, err := q.SQS.makeRequest(req)
	if err != nil {
		return nil, err
	}

	delResponse, errResponse := &DeleteQueueResponse{}, &ErrorResponse{}
	err = unmarshalResponse(httpResponse, delResponse, errResponse)
	if err != nil {
		return nil, err
	}
	return delResponse, nil
}

func (sqs *SQS) GetQueue(queueName string) (queue *Queue, gqResp *GetQueueResponse, err error) {
	vals := sqs.defaultValues("GetQueueUrl")
	vals.Set("QueueName", queueName)
	url := fmt.Sprintf("%v/?%v", sqs.Region.Endpoint, vals.Encode())

	req, err := sign4.NewReusableRequest("GET", url, nil)
	if err != nil {
		return
	}
	httpResp, err := sqs.makeRequest(req)
	if err != nil {
		return
	}

	gqResp, errResponse := &GetQueueResponse{}, &ErrorResponse{}
	err = unmarshalResponse(httpResp, gqResp, errResponse)
	if err != nil {
		return nil, nil, err
	}
	queue = &Queue{SQS: sqs, Name: queueName, Url: gqResp.QueueUrl}
	return
}

//TODO: Get queue for a given account (GetQueueUrl for a given AWS Account ID)

func (sqs *SQS) defaultValues(action string) (vals *url.Values) {
	vals = &url.Values{}
	vals.Set("Action", action)
	vals.Set("AWSAccessKeyId", sqs.Credentials.AccessKey)
	vals.Set("Version", AWS_API_VERSION)
	return
}

func (sqs *SQS) makeRequest(rreq *sign4.ReusableRequest) (resp *http.Response, err error) {
	cred := sqs.Credentials
	hreq, err := rreq.Sign(cred.AccessKey, cred.SecretKey, sqs.Region.Name, SERVICE_NAME)
	if err != nil {
		return
	}

	client := sqs.ClientFactory()
	resp, err = client.Do(hreq)
	return
}

type BodyUnmarshaller interface {
	SetRawResponse(rawResponse []byte)
	SetStatus(status string)
	SetStatusCode(statusCode int)
}

type BodyUnmarshallerError interface {
	BodyUnmarshaller
	error
}

// Try to convert a response to a "good" type.
// Fall back the knownError type.
// Fall back to a generic error if neither of those work
func unmarshalResponse(resp *http.Response, goodResponse BodyUnmarshaller, knownErrResponse BodyUnmarshallerError) (err error) {

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// check first if we have a successful conversion
	err = xml.Unmarshal(body, goodResponse)
	if err == nil {
		goodResponse.SetRawResponse(body)
		goodResponse.SetStatus(resp.Status)
		goodResponse.SetStatusCode(resp.StatusCode)
		return
	}

	err = xml.Unmarshal(body, knownErrResponse)
	if err == nil {
		knownErrResponse.SetRawResponse(body)
		knownErrResponse.SetStatus(resp.Status)
		knownErrResponse.SetStatusCode(resp.StatusCode)
		return knownErrResponse
	}

	return fmt.Errorf("sqs.unmarshalResponse: Unable to unmarshal body data to either %T or %T, Status: %v, body: %s",
		goodResponse, knownErrResponse, resp.Status, body)
}

type CreateQueueResponse struct {
	XMLName     xml.Name `xml:"http://queue.amazonaws.com/doc/2012-11-05/ CreateQueueResponse"`
	QueueUrl    string   `xml:"CreateQueueResult>QueueUrl"`
	RequestId   string   `xml:"ResponseMetadata>RequestId"`
	Status      string
	StatusCode  int
	RawResponse []byte // contains the raw xml data in the response
}

func (cqr *CreateQueueResponse) SetRawResponse(rawResponse []byte) {
	cqr.RawResponse = rawResponse
}

func (cqr *CreateQueueResponse) SetStatus(status string) {
	cqr.Status = status
}

func (cqr *CreateQueueResponse) SetStatusCode(statusCode int) {
	cqr.StatusCode = statusCode
}

type DeleteQueueResponse struct {
	XMLName     xml.Name `xml:"http://queue.amazonaws.com/doc/2012-11-05/ DeleteQueueResponse"`
	RequestId   string   `xml:"ResponseMetadata>RequestId"`
	Status      string
	StatusCode  int
	RawResponse []byte
}

func (dqr *DeleteQueueResponse) SetRawResponse(rawResponse []byte) {
	dqr.RawResponse = rawResponse
}

func (dqr *DeleteQueueResponse) SetStatus(status string) {
	dqr.Status = status
}

func (dqr *DeleteQueueResponse) SetStatusCode(statusCode int) {
	dqr.StatusCode = statusCode
}

type GetQueueResponse struct {
	XMLName     xml.Name `xml:"http://queue.amazonaws.com/doc/2012-11-05/ GetQueueUrlResponse"`
	QueueUrl    string   `xml:"GetQueueUrlResult>QueueUrl"`
	RequestId   string   `xml:"ResponseMetadata>RequestId"`
	Status      string
	StatusCode  int
	RawResponse []byte
}

func (gqr *GetQueueResponse) SetRawResponse(rawResponse []byte) {
	gqr.RawResponse = rawResponse
}

func (gqr *GetQueueResponse) SetStatus(status string) {
	gqr.Status = status
}

func (gqr *GetQueueResponse) SetStatusCode(statusCode int) {
	gqr.StatusCode = statusCode
}

type ErrorResponse struct {
	XMLName     xml.Name  `xml:"http://queue.amazonaws.com/doc/2012-11-05/ ErrorResponse"`
	ErrorInfo   ErrorInfo `xml:"Error"`
	RequestId   string
	Status      string
	StatusCode  int
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

func (e *ErrorResponse) SetStatus(status string) {
	e.Status = status
}

func (e *ErrorResponse) SetStatusCode(statusCode int) {
	e.StatusCode = statusCode
}
