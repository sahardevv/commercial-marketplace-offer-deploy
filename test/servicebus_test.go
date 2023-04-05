package test_test

import (
	//"context"
	//"log"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"testing"
	"time"

	// "github.com/Azure/azure-sdk-for-go/sdk/azcore"
	// "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	// "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	// "github.com/microsoft/commercial-marketplace-offer-deploy/sdk"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/microsoft/commercial-marketplace-offer-deploy/internal/data"
	"github.com/microsoft/commercial-marketplace-offer-deploy/internal/utils"
	"github.com/microsoft/commercial-marketplace-offer-deploy/pkg/messaging"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type serviceBusSuite struct {
	suite.Suite
	ns                  string
	queueName           string
	operationsQueueName string
	subscriptionId      string
	resourceGroupName   string
	location            string
	deploymentName      string
	deploymentId        uint
	invokedOperationId  uint
	db                  data.Database
}

func TestServiceBusSuite(t *testing.T) {
	suite.Run(t, &serviceBusSuite{})
}

func (s *serviceBusSuite) SetupSuite() {
	s.ns = "bobjacmodm.servicebus.windows.net"
	s.queueName = "deployeventqueue"
	s.operationsQueueName = "deployoperationsqueue"
	s.subscriptionId = "31e9f9a0-9fd2-4294-a0a3-0101246d9700"
	s.resourceGroupName = "demo2"
	s.location = "eastus"
	s.deploymentName = "test-deployment"
	s.db = data.NewDatabase(nil)
}

func (s *serviceBusSuite) SetupTest() {
	s.createDeploymentForTests()
}

func (s *serviceBusSuite) createDeploymentForTests() {
	testDeploymentPath := "testdata/taggeddeployment"
	fullPath := filepath.Join(testDeploymentPath, "mainTemplateBicep.json")
	template, err := utils.ReadJson(fullPath)
	require.NoError(s.T(), err)

	paramsPath := filepath.Join(testDeploymentPath, "parameters.json")
	parameters, err := utils.ReadJson(paramsPath)
	require.NoError(s.T(), err)

	deployment := &data.Deployment{
		Name:           s.deploymentName,
		Status:         "New",
		SubscriptionId: s.subscriptionId,
		ResourceGroup:  s.resourceGroupName,
		Location:       s.location,
		Template:       template,
	}

	s.db.Instance().Create(deployment)
	s.deploymentId = deployment.ID

	invokedOperation := &data.InvokedOperation{
		DeploymentId:   deployment.ID,
		DeploymentName: s.deploymentName,
		Params:         parameters,
	}

	s.db.Instance().Create(invokedOperation)
	s.invokedOperationId = invokedOperation.ID
}

func (s *serviceBusSuite) publishTestMessage(sbConfig messaging.ServiceBusConfig, topicHeader string, body string) {
	// sbConfig := messaging.ServiceBusConfig{
	// 	Namespace: s.ns,
	// 	QueueName: s.queueName,
	// }
	config := messaging.PublisherConfig{
		Type:       "servicebus",
		TypeConfig: sbConfig,
	}
	publisher, err := messaging.CreatePublisher(config)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), publisher)
	message := messaging.DeploymentMessage{
		Header: messaging.DeploymentMessageHeader{
			Topic: topicHeader,
		},
		Body: body,
	}
	err = publisher.Publish(message)
	require.NoError(s.T(), err)
}

func (s *serviceBusSuite) TestMessageSendSuccess() {
	sbConfig := messaging.ServiceBusConfig{
		Namespace: s.ns,
		QueueName: s.queueName,
	}
	for i := 0; i < 15; i++ {
		body := fmt.Sprintf("testbody%d", i)
		s.publishTestMessage(sbConfig, "testtopic", body)
	}
}

func (s *serviceBusSuite) TestOperationsSendSuccess() {
	// testDeploymentPath := "testdata/taggeddeployment"
	// fullPath := filepath.Join(testDeploymentPath, "mainTemplateBicep.json")
	// template, err := utils.ReadJson(fullPath)
	// require.NoError(s.T(), err)

	// paramsPath := filepath.Join(testDeploymentPath, "parameters.json")
	// parameters, err := utils.ReadJson(paramsPath)
	// require.NoError(s.T(), err)

	var dataDeployment data.Deployment
	s.db.Instance().First(&dataDeployment, s.deploymentId)

	var invokedOperation data.InvokedOperation
	s.db.Instance().First(&invokedOperation, s.invokedOperationId)

	// azureDeployment := deployment.AzureDeployment{
	// 	SubscriptionId:    s.subscriptionId,
	// 	Location:          s.location,
	// 	ResourceGroupName: s.resourceGroupName,
	// 	DeploymentName:    "operationsTest",
	// 	Template:          dataDeployment.Template,
	// 	Params:            invokedOperation.Params,
	// }

	bodyByte, err := json.Marshal(invokedOperation)
	require.NoError(s.T(), err)

	bodyString := string(bodyByte)
	sbConfig := messaging.ServiceBusConfig{
		Namespace: s.ns,
		QueueName: s.operationsQueueName,
	}

	s.publishTestMessage(sbConfig, "testtopic", bodyString)
}

func (s *serviceBusSuite) TestMessageReceiveSuccess() {
	sbConfig := messaging.ServiceBusConfig{
		Namespace: s.ns,
		QueueName: s.queueName,
	}

	handler := &fakeHandler{}

	receiver, err := messaging.NewServiceBusReceiver(sbConfig, handler)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), receiver)
	fmt.Println("calling start")
	go receiver.Start()
	// sleep for 5 seconds to allow the receiver to start
	fmt.Println("starting sleep 1")
	time.Sleep(5 * time.Second)
	go receiver.Stop()
	fmt.Println("After the stop in TestMessageReceiveSuccess")
	fmt.Println("Starting sleep 2")
	time.Sleep(5 * time.Second)
	fmt.Println("After the second sleep")
}

func (s *serviceBusSuite) TestOperationDeploymentSuccess() {
	sbConfig := messaging.ServiceBusConfig{
		Namespace: s.ns,
		QueueName: s.queueName,
	}

	handler := messaging.NewOperationsHandler()
	receiver, err := messaging.NewServiceBusReceiver(sbConfig, handler)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), receiver)
	fmt.Println("calling start")
	go receiver.Start()
	time.Sleep(5 * time.Second)
}

type fakeHandler struct {
}

func (h *fakeHandler) Handle(ctx context.Context, message *azservicebus.ReceivedMessage) error {
	log.Println("Handling message")
	return nil
}
