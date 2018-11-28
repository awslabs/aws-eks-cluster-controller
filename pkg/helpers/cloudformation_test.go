package helpers

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"reflect"
	"testing"
)

type mockCloudformationAPI struct {
	cloudformationiface.CloudFormationAPI
	throwCreateStackErr    bool
	throwWaitErr           bool
	throwDescribeStacksErr bool
	throwDeleteStackErr    bool
}

func (m *mockCloudformationAPI) CreateStack(input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	if m.throwCreateStackErr {
		return nil, errors.New("foo")
	}
	return &cloudformation.CreateStackOutput{
		StackId: aws.String("foo"),
	}, nil
}

func (m *mockCloudformationAPI) WaitUntilStackCreateComplete(input *cloudformation.DescribeStacksInput) error {
	if m.throwWaitErr {
		return errors.New("foo")
	}
	return nil
}

func (m *mockCloudformationAPI) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
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

func (m *mockCloudformationAPI) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	if m.throwDeleteStackErr {
		return nil, errors.New("foo")
	}
	return &cloudformation.DeleteStackOutput{}, nil
}

func (m *mockCloudformationAPI) WaitUntilStackDeleteComplete(input *cloudformation.DescribeStacksInput) error {
	if m.throwWaitErr {
		return errors.New("foo")
	}
	return nil
}

func TestCreateAndDescribeStack(t *testing.T) {
	type args struct {
		cfnSvc *mockCloudformationAPI
		input  *cloudformation.CreateStackInput
	}

	tests := []struct {
		name string
		args args
		want *cloudformation.Stack
		err  error
	}{
		{
			name: "create stack successful",
			args: args{
				cfnSvc: &mockCloudformationAPI{},
				input: &cloudformation.CreateStackInput{
					StackName:    aws.String("foo"),
					TemplateBody: aws.String("anybody"),
				},
			},
			want: &cloudformation.Stack{
				StackName:   aws.String("foo"),
				StackStatus: aws.String("bar"),
			},
			err: nil,
		},
		{
			name: "create stack error",
			args: args{
				cfnSvc: &mockCloudformationAPI{throwCreateStackErr: true},
				input: &cloudformation.CreateStackInput{
					StackName:    aws.String("foo"),
					TemplateBody: aws.String("anybody"),
				},
			},
			want: nil,
			err:  fmt.Errorf("unable to create stack foo: foo"),
		},
		{
			name: "create stack wait error",
			args: args{
				cfnSvc: &mockCloudformationAPI{throwWaitErr: true},
				input: &cloudformation.CreateStackInput{
					StackName:    aws.String("foo"),
					TemplateBody: aws.String("anybody"),
				},
			},
			want: nil,
			err:  fmt.Errorf("error waiting for stack to complete foo: foo"),
		},
		{
			name: "create stack describe stack error",
			args: args{
				cfnSvc: &mockCloudformationAPI{throwDescribeStacksErr: true},
				input: &cloudformation.CreateStackInput{
					StackName:    aws.String("foo"),
					TemplateBody: aws.String("anybody"),
				},
			},
			want: nil,
			err:  fmt.Errorf("error describing stack foo: foo"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CreateAndDescribeStack(tc.args.cfnSvc, tc.args.input)
			if !reflect.DeepEqual(tc.want, got) {
				t.Errorf("Expected: \n%+v\n\n Got:\n%+v", tc.want, got)
			}
			if err != nil && tc.err.Error() != err.Error() {
				t.Errorf("Expected:%+v Got:%+v", tc.err, err)
			}
		})
	}
}

func TestDeleteStack(t *testing.T) {
	type args struct {
		cfnSvc    *mockCloudformationAPI
		stackName string
	}

	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "delete stack successful",
			args: args{
				cfnSvc:    &mockCloudformationAPI{},
				stackName: "somename",
			},
			err: nil,
		},
		{
			name: "delete stack error",
			args: args{
				cfnSvc:    &mockCloudformationAPI{throwDeleteStackErr: true},
				stackName: "somename",
			},
			err: fmt.Errorf("error deleting stack somename: foo"),
		},
		{
			name: "delete stack wait error",
			args: args{
				cfnSvc:    &mockCloudformationAPI{throwWaitErr: true},
				stackName: "somename",
			},
			err: fmt.Errorf("error watiting for stack to delete somename: foo"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := DeleteStack(tc.args.cfnSvc, tc.args.stackName)
			if err != nil && tc.err.Error() != err.Error() {
				t.Errorf("Expected:%+v Got:%+v", tc.err, err)
			}
		})
	}
}
