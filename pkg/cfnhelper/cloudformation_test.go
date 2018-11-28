package cfnhelper

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"reflect"
	"testing"
)

func TestCreateAndDescribeStack(t *testing.T) {
	type args struct {
		cfnSvc *MockCloudformationAPI
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
				cfnSvc: &MockCloudformationAPI{},
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
				cfnSvc: &MockCloudformationAPI{throwCreateStackErr: true},
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
				cfnSvc: &MockCloudformationAPI{throwWaitErr: true},
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
				cfnSvc: &MockCloudformationAPI{throwDescribeStacksErr: true},
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
		cfnSvc    *MockCloudformationAPI
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
				cfnSvc:    &MockCloudformationAPI{},
				stackName: "somename",
			},
			err: nil,
		},
		{
			name: "delete stack error",
			args: args{
				cfnSvc:    &MockCloudformationAPI{throwDeleteStackErr: true},
				stackName: "somename",
			},
			err: fmt.Errorf("error deleting stack somename: foo"),
		},
		{
			name: "delete stack wait error",
			args: args{
				cfnSvc:    &MockCloudformationAPI{throwWaitErr: true},
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
