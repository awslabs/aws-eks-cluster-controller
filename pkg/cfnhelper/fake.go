package cfnhelper

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

// MockCloudformationAPI provides mocked interface to AWS Cloudformation service
type MockCloudformationAPI struct {
	cloudformationiface.CloudFormationAPI

	Err    error
	Status string

	FailCreate   bool
	FailDescribe bool
	FailDelete   bool

	ResetDescribe bool
}

func (m *MockCloudformationAPI) CreateStack(input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if m.FailCreate {
		return nil, m.Err
	}

	if m.ResetDescribe {
		m.FailDescribe = false
	}

	return &cloudformation.CreateStackOutput{
		StackId: aws.String("foo"),
	}, nil
}

func (m *MockCloudformationAPI) WaitUntilStackCreateComplete(input *cloudformation.DescribeStacksInput) error {
	return nil
}

func (m *MockCloudformationAPI) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if m.FailDescribe {
		return nil, m.Err
	}

	return &cloudformation.DescribeStacksOutput{
		Stacks: []*cloudformation.Stack{
			{
				StackName:   aws.String("foo"),
				StackStatus: aws.String(m.Status),
			},
		},
	}, nil
}

func (m *MockCloudformationAPI) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if m.FailDelete {
		return nil, m.Err
	}
	return &cloudformation.DeleteStackOutput{}, nil
}

func (m *MockCloudformationAPI) WaitUntilStackDeleteComplete(input *cloudformation.DescribeStacksInput) error {
	return nil
}