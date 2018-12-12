package cfnhelper

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

func CreateAndDescribeStack(cfnSvc cloudformationiface.CloudFormationAPI, input *cloudformation.CreateStackInput) (*cloudformation.Stack, error) {
	if _, err := cfnSvc.CreateStack(input); err != nil {
		return nil, fmt.Errorf("unable to create stack %s: %v", *input.StackName, err)
	}

	if err := cfnSvc.WaitUntilStackCreateComplete(&cloudformation.DescribeStacksInput{
		StackName: input.StackName,
	}); err != nil {
		return nil, fmt.Errorf("error waiting for stack to complete %s: %v", *input.StackName, err)
	}

	return DescribeStack(cfnSvc, *input.StackName)
}

func IsDoesNotExist(err error, stackName string) bool {
	if err != nil {
		if aErr, ok := err.(awserr.Error); ok {
			if aErr.Code() == "ValidationError" && aErr.Message() == fmt.Sprintf("Stack with id %s does not exist", stackName) {
				return true
			}
		}

	}
	return false
}

func DescribeStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName string) (*cloudformation.Stack, error) {
	out, err := cfnSvc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, err
	}

	return out.Stacks[0], nil
}

func DeleteStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName string) error {
	_, err := cfnSvc.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("error deleting stack %s: %v", stackName, err)
	}

	err = cfnSvc.WaitUntilStackDeleteComplete(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("error watiting for stack to delete %s: %v", stackName, err)
	}

	return nil
}
