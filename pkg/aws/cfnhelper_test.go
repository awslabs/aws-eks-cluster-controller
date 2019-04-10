package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

func TestGetCFNTemplateBody(t *testing.T) {

	type want struct {
		shouldError bool
		body        string
	}

	type testCase struct {
		name     string
		template string
		input    interface{}
		expected want
	}

	testCases := []testCase{
		{name: "returns error if template invalid", template: `{{`, input: map[string]string{}, expected: want{shouldError: true, body: ""}},
		{name: "returns error if missing values", template: `{{.foo}}`, input: map[string]string{}, expected: want{shouldError: true, body: ""}},
		{name: "renders simple template", template: `{{.foo}}`, input: map[string]string{"foo": "bar"}, expected: want{shouldError: false, body: "bar"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			body, err := GetCFNTemplateBody(tc.template, tc.input)
			if tc.expected.shouldError && err == nil {
				t.Errorf(`Expected err != nil`)
			}

			if !tc.expected.shouldError && err != nil {
				t.Errorf(`Expected err == nil, Got %v`, err)
			}

			if body != tc.expected.body {
				t.Errorf(`Expected body == "%s", Got "%s"`, tc.expected.body, body)
			}
		})
	}
}

func TestStackDoesNotExist(t *testing.T) {
	type testCase struct {
		name     string
		inputErr error
		expected bool
	}

	testCases := []testCase{
		{
			name:     "return true if status Code is 400 and error Type is Validation Error",
			inputErr: awserr.New("ValidationError", `ValidationError: Stack with id foo does not exist, status code: 400, request id: 8f05552d-3957`, nil),
			expected: true,
		},
		{
			name:     "return false if status Code is not 400",
			inputErr: awserr.New("ValidationError", `ValidationError: Stack with id foo does not exist, status code: 500, request id: 8f05552d-3957`, nil),
			expected: false,
		},
		{
			name:     "return false if Code is not ValidationError",
			inputErr: awserr.New("AccessDeniedException", `blah`, nil),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsStackDoesNotExist(tc.inputErr)
			if got != tc.expected {
				t.Errorf("Expected %t for input %v : got %t", tc.expected, tc.inputErr, got)
			}
		})
	}

}
