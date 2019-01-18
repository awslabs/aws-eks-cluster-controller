package cfnhelper

import (
	"testing"
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
		{name: "renders quoted list of strings",
			template: `{{quoteList .bar}}`,
			input:    map[string][]string{"bar": []string{"foo", "bar"}},
			expected: want{shouldError: false, body: `["foo", "bar"]`},
		},
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
