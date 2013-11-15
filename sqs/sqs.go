package sqs

import (
	//"bytes"
	"encoding/xml"
	"fmt"
	"github.com/p-lewis/awsgolang/auth"
	"github.com/p-lewis/awsgolang/sign4"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
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

	cqResponse = &CreateQueueResponse{}
	err = sqs.getResults(sqs.Region.Endpoint, vals, nil, cqResponse)

	if err != nil {
		return nil, nil, err
	}

	sqsQueue = &Queue{SQS: sqs, Name: name, Url: cqResponse.QueueUrl}

	return
}

func (q *Queue) DeleteQueue() (*DeleteQueueResponse, error) {
	vals := q.SQS.defaultValues("DeleteQueue")
	delResponse := &DeleteQueueResponse{}
	err := q.SQS.getResults(q.Url, vals, nil, delResponse)
	if err != nil {
		return nil, err
	}
	return delResponse, nil
}

// Get queue for a given name and AWS Account ID.
// If accountId is an empty string (""), returns queues for the current requesting account.
func (sqs *SQS) GetQueue(queueName, accountId string) (queue *Queue, gqResp *GetQueueResponse, err error) {
	vals := sqs.defaultValues("GetQueueUrl")
	vals.Set("QueueName", queueName)
	if accountId != "" {
		vals.Set("QueueOwnerAWSAccountId", accountId)
	}
	gqResp = &GetQueueResponse{}
	err = sqs.getResults(sqs.Region.Endpoint, vals, nil, gqResp)

	if err != nil {
		return nil, nil, err
	}
	queue = &Queue{SQS: sqs, Name: queueName, Url: gqResp.QueueUrl}
	return
}

//List queues. If queueNamePrefix not empty (i.e. not ""), only queues with a name beginning
// with the specified value are returned.
func (sqs *SQS) ListQueues(queueNamePrefix string) (queues []Queue, lqResp *ListQueuesResponse, err error) {
	vals := sqs.defaultValues("ListQueues")
	if queueNamePrefix != "" {
		vals.Set("QueueNamePrefix", queueNamePrefix)
	}
	lqResp = &ListQueuesResponse{}
	err = sqs.getResults(sqs.Region.Endpoint, vals, nil, lqResp)
	if err != nil {
		return nil, nil, err
	}

	queues = make([]Queue, len(lqResp.QueueUrls))
	for i, url := range lqResp.QueueUrls {
		_, name := path.Split(url)
		queues[i] = Queue{SQS: sqs, Name: name, Url: url}
	}
	return
}

// GET results for a given uri, values, expected.
func (sqs *SQS) getResults(uri string, values *url.Values, body io.Reader, goodResponse BodyUnmarshaller) (err error) {
	url := fmt.Sprintf("%v/?%v", uri, values.Encode())
	req, err := sign4.NewReusableRequest("GET", url, body)
	if err != nil {
		return
	}
	httpResp, err := sqs.makeRequest(req)
	if err != nil {
		return
	}
	errResponse := &ErrorResponse{}
	err = unmarshalResponse(httpResp, goodResponse, errResponse)
	return
}

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

type BodyUnmarshaller interface {
	SetRawResponse(rawResponse []byte)
	SetStatus(status string)
	SetStatusCode(statusCode int)
}

type BodyUnmarshallerError interface {
	BodyUnmarshaller
	error
}

type AWSResponse struct {
	Status      string
	StatusCode  int
	RawResponse []byte // contains the raw xml data in the response
}

func (r *AWSResponse) SetRawResponse(rawResponse []byte) {
	r.RawResponse = rawResponse
}

func (r *AWSResponse) SetStatus(status string) {
	r.Status = status
}

func (r *AWSResponse) SetStatusCode(statusCode int) {
	r.StatusCode = statusCode
}

type CreateQueueResponse struct {
	XMLName   xml.Name `xml:"CreateQueueResponse"` //http://queue.amazonaws.com/doc/2012-11-05/
	QueueUrl  string   `xml:"CreateQueueResult>QueueUrl"`
	RequestId string   `xml:"ResponseMetadata>RequestId"`
	AWSResponse
}

type DeleteQueueResponse struct {
	XMLName   xml.Name `xml:"DeleteQueueResponse"` //http://queue.amazonaws.com/doc/2012-11-05/
	RequestId string   `xml:"ResponseMetadata>RequestId"`
	AWSResponse
}

type GetQueueResponse struct {
	XMLName   xml.Name `xml:"GetQueueUrlResponse"` //http://queue.amazonaws.com/doc/2012-11-05/
	QueueUrl  string   `xml:"GetQueueUrlResult>QueueUrl"`
	RequestId string   `xml:"ResponseMetadata>RequestId"`
	AWSResponse
}

type ListQueuesResponse struct {
	XMLName   xml.Name `xml:"ListQueuesResponse"` //http://queue.amazonaws.com/doc/2012-11-05/
	QueueUrls []string `xml:"ListQueuesResult>QueueUrl"`
	RequestId string   `xml:"ResponseMetadata>RequestId"`
	AWSResponse
}

type ErrorResponse struct {
	XMLName   xml.Name  `xml:"ErrorResponse"` //http://queue.amazonaws.com/doc/2012-11-05/
	Err       ErrorInfo `xml:"Error"`
	RequestId string
	AWSResponse
}

type ErrorInfo struct {
	Type, Code, Message, Detail string
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("sqs.ErrorResponse Type: %v, Code: %v Message: %v",
		e.Err.Type, e.Err.Code, e.Err.Message)
}
