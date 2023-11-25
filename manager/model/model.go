package model

import (
	"fmt"
	"regexp"

	api "github.com/onpremless/go-client"
)

func ValidateCreateLambda(lambda *api.CreateLambda) error {
	if lambda.Name == "" {
		return fmt.Errorf("'name' is required")
	}

	if lambda.Runtime == "" {
		return fmt.Errorf("'runtime' is required")
	}

	if lambda.LambdaType == "" {
		return fmt.Errorf("'lambda_type' is required")
	}

	if lambda.LambdaType != "ENDPOINT" && lambda.LambdaType != "INTERNAL" {
		return fmt.Errorf("invalid 'lambda_type' value: %s", lambda.LambdaType)
	}

	return nil
}

var EndpointRegex = regexp.MustCompile("^(/[0-9a-zA-Z-_]+)+$")

func ValidateEndpoint(path string) error {
	if !EndpointRegex.MatchString(path) {
		return fmt.Errorf("'endpoint' doesn't conform regex: %s", EndpointRegex.String())
	}

	return nil
}

func ValidateCreateRuntime(req *api.CreateRuntime) error {
	if req.Name == "" {
		return fmt.Errorf("'name' is required")
	}

	if req.Dockerfile == "" {
		return fmt.Errorf("'dockerfile' is required")
	}

	return nil
}

func ValidateCreateEndpoint(req *api.CreateEndpoint) error {
	if req.Name == "" {
		return fmt.Errorf("'name' is required")
	}

	if req.Lambda == "" {
		return fmt.Errorf("'lambda' is required")
	}

	if err := ValidateEndpoint(req.Path); err != nil {
		return err
	}

	return nil
}
