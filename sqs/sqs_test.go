package sqs_test

import (
	//"fmt"
	. "launchpad.net/gocheck"
	"testing"

	// "bufio"
	// "bytes"
	// "errors"
	"flag"
	"github.com/p-lewis/awsgolang/auth"
	"github.com/p-lewis/awsgolang/sqs"
	// "io/ioutil"
	// "net/http"
	// "path/filepath"
	// "strings"
	"time"
)

//var testSuiteDir = flag.String("test-suite-dir", "", "Directory containing the aws4_testsute files.")

func Test(t *testing.T) { TestingT(t) }

type SQSSuite struct {
	// request1 *sign4.ReusableRequest
	// request2 *sign4.ReusableRequest
}

var _ = Suite(&SQSSuite{})

var testRegion = &sqs.Region{Name: "test-region", Endpoint: "http://localhost:6924/testendpoint"}
var testCredentials = &auth.Credentials{AccessKey: "WHOAMI", SecretKey: "ITSASECRET"}
var testSQS = sqs.SQS{testCredentials, testRegion, sqs.DefaultClientFactory}

// func (s *SQSSuite) TestCreateQueue(c *C) {
// 	testSQS.CreateQueue("TestQueue")
// }

// LIVE tests; will cost $$ if you run!

type LiveSQSSuite struct {
	Credentials *auth.Credentials
	SQS         *sqs.SQS
	TestQueue   *sqs.Queue
}

var _ = Suite(&LiveSQSSuite{})

var live = flag.Bool("sqs.live", false, "Include live tests (can cost real money).")

func (s *LiveSQSSuite) SetUpSuite(c *C) {
	if !*live {
		c.Skip("-sqs.live not provided, skipping live SQS tests.")
		return
	}
	cred, err := auth.EnvCredentials()
	if err != nil {
		c.Skip("Could not get environment credentials, skipping live tests. Error: " + err.Error())
		return
	}
	s.Credentials = cred
	s.SQS = &sqs.SQS{s.Credentials, &sqs.USWest, sqs.DefaultClientFactory}

	testQueue, _, err := s.createLiveQueue("Test_sqs_test_LiveTestQueue_" + time.Now().Format(TIMESTAMP_FMT))
	if err != nil {
		c.Log(err)
		c.Skip("Could not create a test queue in SetUpSuite, skipping live tests")
		return
	}
	s.TestQueue = testQueue
}

func (s *LiveSQSSuite) TearDownSuite(c *C) {
	s.TestQueue.DeleteQueue()
}

const (
	TIMESTAMP_FMT = "2006-01-02T15-04-05MST"
)

func (s *LiveSQSSuite) TestLiveCreateQueue(c *C) {
	queueName := "Test_sqs_test_CreateQueue_" + time.Now().Format(TIMESTAMP_FMT)
	queue, cResp, err := s.createLiveQueue(queueName)
	c.Assert(err, IsNil)
	c.Assert(queue.Name, Equals, queueName)
	c.Assert(queue.Url, Not(Equals), "")
	c.Assert(queue.Url, Equals, cResp.QueueUrl)
	c.Assert(cResp.RequestId, Not(Equals), "")
	c.Assert(cResp.Status, Equals, "200 OK")
	c.Assert(cResp.StatusCode, Equals, 200)
}

func (s *LiveSQSSuite) TestLiveCreateQueueFailure(c *C) {
	queueName := "Test_sqs_test_83*A111"
	queue, cResp, err := s.createLiveQueue(queueName)
	c.Assert(queue, IsNil)
	c.Assert(cResp, IsNil)
	errResp, ok := err.(*sqs.ErrorResponse)
	c.Assert(ok, Equals, true)
	c.Assert(errResp.ErrorInfo.Type, Equals, "Sender")
	c.Assert(errResp.ErrorInfo.Code, Equals, "InvalidParameterValue")
	c.Assert(errResp.Status, Equals, "400 Bad Request")
	c.Assert(errResp.StatusCode, Equals, 400)
}

func (s *LiveSQSSuite) TestLiveDeleteQueue(c *C) {
	queueName := "Test_sqs_testDeleteQueue_" + time.Now().Format(TIMESTAMP_FMT)
	queue, _, err := s.createLiveQueue(queueName)
	c.Assert(err, IsNil)
	delResp, err := queue.DeleteQueue()
	c.Assert(err, IsNil)
	//fmt.Println("RequestId:", delResp.RequestId)
	c.Assert(err, IsNil)
	c.Assert(delResp.RequestId, Not(Equals), "")
	c.Assert(delResp.Status, Equals, "200 OK")
	c.Assert(delResp.StatusCode, Equals, 200)
}

func (s *LiveSQSSuite) TestLiveDeleteNonexistentQueue(c *C) {

	fakeQueue := &sqs.Queue{
		SQS:  s.SQS,
		Name: "Test_sqs_test_DeleteNonexistentQueue",
		Url:  "https://sqs.us-west-1.amazonaws.com/159365254521/Test_sqs_test_DeleteNonexistentQueue"}
	delResp, err := fakeQueue.DeleteQueue()
	c.Assert(delResp, IsNil)
	c.Assert(err, Not(IsNil))
	errResponse, ok := err.(*sqs.ErrorResponse)
	c.Assert(ok, Equals, true)
	c.Assert(errResponse.ErrorInfo.Type, Equals, "Sender")
	c.Assert(errResponse.ErrorInfo.Code, Equals, "AWS.SimpleQueueService.NonExistentQueue")
	c.Assert(errResponse.Status, Equals, "400 Bad Request")
	c.Assert(errResponse.StatusCode, Equals, 400)
	//fmt.Printf("RawResponse:\n%s\n", .RawResponse)

}

func (s *LiveSQSSuite) TestLiveGetQueue(c *C) {
	name := s.TestQueue.Name
	queue, gqResp, err := s.SQS.GetQueue(name)
	c.Assert(err, IsNil)
	c.Assert(gqResp.Status, Equals, "200 OK")
	c.Assert(gqResp.StatusCode, Equals, 200)
	c.Assert(queue.Url, Equals, s.TestQueue.Url)
	c.Assert(queue.Name, Equals, name)

}

func (s *LiveSQSSuite) createLiveQueue(name string) (*sqs.Queue, *sqs.CreateQueueResponse, error) {
	return s.SQS.CreateQueue(name)
}
