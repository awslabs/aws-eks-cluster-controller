package cfnhelper

import (
	"bytes"
	"fmt"
	"strings"

	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

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

//GetCFNTemplateBody renders a Cloudformation template
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
