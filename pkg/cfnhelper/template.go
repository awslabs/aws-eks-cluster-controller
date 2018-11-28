package cfnhelper

import (
	"bytes"
	"text/template"
)

func GetCFNTemplateBody(filename string, m map[string]string) (string, error) {
	controlPlaneTemplate, err := template.ParseFiles(filename)
	if err != nil {
		return "", err
	}
	b := bytes.NewBuffer([]byte{})
	if err := controlPlaneTemplate.Execute(b, m); err != nil {
		return "", err
	}
	return b.String(), nil
}
