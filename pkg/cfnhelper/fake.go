package cfnhelper

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

// MockCloudformationAPI provides mocked interface to AWS Cloudformation service
type MockCloudformationAPI struct {
	cloudformationiface.CloudFormationAPI
	throwCreateStackErr    bool
	throwWaitErr           bool
	throwDescribeStacksErr bool
	throwDeleteStackErr    bool
}

func (m *MockCloudformationAPI) CreateStack(input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if m.throwCreateStackErr {
		return nil, errors.New("foo")
	}
	return &cloudformation.CreateStackOutput{
		StackId: aws.String("foo"),
	}, nil
}

func (m *MockCloudformationAPI) WaitUntilStackCreateComplete(input *cloudformation.DescribeStacksInput) error {
	if m.throwWaitErr {
		return errors.New("foo")
	}
	return nil
}

func (m *MockCloudformationAPI) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	if m.throwDescribeStacksErr {
		return nil, errors.New("foo")
	}
	return &cloudformation.DescribeStacksOutput{
		Stacks: []*cloudformation.Stack{
			{
				StackName:   aws.String("foo"),
				StackStatus: aws.String("bar"),
			},
		},
	}, nil
}

func (m *MockCloudformationAPI) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if m.throwDeleteStackErr {
		return nil, errors.New("foo")
	}
	return &cloudformation.DeleteStackOutput{}, nil
}

func (m *MockCloudformationAPI) WaitUntilStackDeleteComplete(input *cloudformation.DescribeStacksInput) error {
	if m.throwWaitErr {
		return errors.New("foo")
	}
	return nil
}
