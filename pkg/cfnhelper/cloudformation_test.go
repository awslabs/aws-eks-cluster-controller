package cfnhelper

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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
				cfnSvc: &MockCloudformationAPI{Status: "bar"},
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
				cfnSvc: &MockCloudformationAPI{FailCreate: true, Err: fmt.Errorf("foo")},
				input: &cloudformation.CreateStackInput{
					StackName:    aws.String("foo"),
					TemplateBody: aws.String("anybody"),
				},
			},
			want: nil,
			err:  fmt.Errorf("unable to create stack foo: foo"),
		},
		{
			name: "create stack describe stack error",
			args: args{
				cfnSvc: &MockCloudformationAPI{FailDescribe: true, Err: fmt.Errorf("foo")},
				input: &cloudformation.CreateStackInput{
					StackName:    aws.String("foo"),
					TemplateBody: aws.String("anybody"),
				},
			},
			want: nil,
			err:  fmt.Errorf("foo"),
		},
		{
			name: "ResetDescribe Makes DescribeStacks return true in create.",
			args: args{
				cfnSvc: &MockCloudformationAPI{FailDescribe: true, ResetDescribe: true, Status: "bar"},
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
				cfnSvc:    &MockCloudformationAPI{FailDelete: true, Err: fmt.Errorf("foo")},
				stackName: "somename",
			},
			err: fmt.Errorf("error deleting stack somename: foo"),
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
