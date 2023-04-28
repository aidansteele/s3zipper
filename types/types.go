package types

import "encoding/json"

type GetObjectInput struct {
	XAmzRequestId    string          `json:"xAmzRequestId"`
	Configuration    Configuration   `json:"configuration"`
	UserRequest      UserRequest     `json:"userRequest"`
	UserIdentity     json.RawMessage `json:"userIdentity"`
	ProtocolVersion  string          `json:"protocolVersion"`
	GetObjectContext struct {
		InputS3Url  string `json:"inputS3Url"`
		OutputRoute string `json:"outputRoute"`
		OutputToken string `json:"outputToken"`
	} `json:"getObjectContext"`
}

type GetObjectOutput struct {
	StatusCode int `json:"statusCode"`
}

type HeadObjectInput struct {
	XAmzRequestId     string          `json:"xAmzRequestId"`
	Configuration     Configuration   `json:"configuration"`
	UserRequest       UserRequest     `json:"userRequest"`
	UserIdentity      json.RawMessage `json:"userIdentity"`
	ProtocolVersion   string          `json:"protocolVersion"`
	HeadObjectContext struct {
		InputS3Url string `json:"inputS3Url"`
	} `json:"headObjectContext"`
}

type HeadObjectOutput struct {
	StatusCode   int               `json:"statusCode"`
	ErrorCode    string            `json:"errorCode,omitempty"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
	Headers      map[string]string `json:"headers"`
}

type Configuration struct {
	AccessPointArn           string `json:"accessPointArn"`
	SupportingAccessPointArn string `json:"supportingAccessPointArn"`
	Payload                  string `json:"payload"`
}

type UserRequest struct {
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}
