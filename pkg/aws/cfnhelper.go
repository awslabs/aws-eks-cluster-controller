package aws

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

//CompleteStatuses contains all CloudFormation status strings that we consider to be complete for a vpn
var CompleteStatuses = []string{
	cloudformation.StackStatusCreateComplete,
	cloudformation.StackStatusUpdateComplete,
	cloudformation.StackStatusDeleteComplete,
}

//FailedStatuses contains all CloudFormation status strings that we consider to be failed for a vpn
var FailedStatuses = []string{
	cloudformation.StackStatusCreateFailed,
	cloudformation.StackStatusRollbackComplete,
	cloudformation.StackStatusRollbackFailed,
	cloudformation.StackStatusUpdateRollbackFailed,
	cloudformation.StackStatusUpdateRollbackComplete,
	cloudformation.StackStatusDeleteFailed,
}

//PendingStatuses contains all CloudFormation status strings that we consider to be pending for a vpn
var PendingStatuses = []string{
	cloudformation.StackStatusCreateInProgress,
	cloudformation.StackStatusDeleteInProgress,
	cloudformation.StackStatusRollbackInProgress,
	cloudformation.StackStatusUpdateCompleteCleanupInProgress,
	cloudformation.StackStatusUpdateInProgress,
	cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
	cloudformation.StackStatusUpdateRollbackInProgress,
	cloudformation.StackStatusReviewInProgress,
}

// IsFailed tests if the specified string is considered a failed cloudformation stack state
func IsFailed(status string) bool {
	for _, s := range FailedStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsComplete tests if the specified string is considered a completed cloudformation stack state
func IsComplete(status string) bool {
	for _, s := range CompleteStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsPending tests if the specified string is considered a pending cloudformation stack state
func IsPending(status string) bool {
	for _, s := range PendingStatuses {
		if s == status {
			return true
		}
	}
	return false
}

//StackDoesNotExist Checks if the error recieved for DescribeStacks denotes if the stack is non exsistent
func StackDoesNotExist(err error) bool {
	if aErr, ok := err.(awserr.Error); ok {
		matched, _ := regexp.MatchString(`status code: 400`, aErr.Error())
		if aErr.Code() == "ValidationError" {
			return matched
		}
	}
	return false
}

//DescribeStack takes a stackName as input and returns the Stack information along with any errors.
func DescribeStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName string) (*cloudformation.Stack, error) {
	out, err := cfnSvc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, err
	}

	return out.Stacks[0], nil
}

//GetCFNTemplateBody takes a cfnTemplate and a corresponding struct as input and return the rendered template
func GetCFNTemplateBody(cfnTemplate string, input interface{}) (string, error) {
	templatizedCFN, err := template.New("cfntemplate").Option("missingkey=error").Funcs(template.FuncMap{
		"quoteList": func(s []string) string {
			return fmt.Sprintf(`["%s"]`, strings.Join(s, `", "`))
		}}).Parse(cfnTemplate)

	if err != nil {
		return "", err
	}
	b := bytes.NewBuffer([]byte{})
	if err := templatizedCFN.Execute(b, input); err != nil {
		return "", err
	}
	return b.String(), nil
}
