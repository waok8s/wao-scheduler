// Package api provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.15.0 DO NOT EDIT.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/oapi-codegen/runtime"
)

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Doer performs HTTP requests.
//
// The standard http.Client implements this interface.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client which conforms to the OpenAPI3 specification for this service.
type Client struct {
	// The endpoint of the server conforming to this interface, with scheme,
	// https://api.deepmap.com for example. This can contain a path relative
	// to the server, such as https://api.deepmap.com/dev-test, and all the
	// paths in the swagger spec will be appended to the server.
	Server string

	// Doer for performing requests, typically a *http.Client with any
	// customized settings, such as certificate chains.
	Client HttpRequestDoer

	// A list of callbacks for modifying requests which are generated before sending over
	// the network.
	RequestEditors []RequestEditorFn
}

// ClientOption allows setting custom parameters during construction
type ClientOption func(*Client) error

// Creates a new Client, with reasonable defaults
func NewClient(server string, opts ...ClientOption) (*Client, error) {
	// create a client with sane default values
	client := Client{
		Server: server,
	}
	// mutate client and add all optional params
	for _, o := range opts {
		if err := o(&client); err != nil {
			return nil, err
		}
	}
	// ensure the server URL always has a trailing slash
	if !strings.HasSuffix(client.Server, "/") {
		client.Server += "/"
	}
	// create httpClient, if not already present
	if client.Client == nil {
		client.Client = &http.Client{}
	}
	return &client, nil
}

// WithHTTPClient allows overriding the default Doer, which is
// automatically created using http.Client. This is useful for tests.
func WithHTTPClient(doer HttpRequestDoer) ClientOption {
	return func(c *Client) error {
		c.Client = doer
		return nil
	}
}

// WithRequestEditorFn allows setting up a callback function, which will be
// called right before sending the request. This can be used to mutate the request.
func WithRequestEditorFn(fn RequestEditorFn) ClientOption {
	return func(c *Client) error {
		c.RequestEditors = append(c.RequestEditors, fn)
		return nil
	}
}

// The interface specification for the client above.
type ClientInterface interface {
	// GetRedfishV1SystemsSystemId request
	GetRedfishV1SystemsSystemId(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*http.Response, error)

	// GetRedfishV1SystemsSystemIdMachineLearningModel request
	GetRedfishV1SystemsSystemIdMachineLearningModel(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*http.Response, error)
}

func (c *Client) GetRedfishV1SystemsSystemId(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetRedfishV1SystemsSystemIdRequest(c.Server, systemId)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) GetRedfishV1SystemsSystemIdMachineLearningModel(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetRedfishV1SystemsSystemIdMachineLearningModelRequest(c.Server, systemId)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetRedfishV1SystemsSystemIdRequest generates requests for GetRedfishV1SystemsSystemId
func NewGetRedfishV1SystemsSystemIdRequest(server string, systemId string) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "systemId", runtime.ParamLocationPath, systemId)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/redfish/v1/Systems/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewGetRedfishV1SystemsSystemIdMachineLearningModelRequest generates requests for GetRedfishV1SystemsSystemIdMachineLearningModel
func NewGetRedfishV1SystemsSystemIdMachineLearningModelRequest(server string, systemId string) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "systemId", runtime.ParamLocationPath, systemId)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/redfish/v1/Systems/%s/MachineLearningModel", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *Client) applyEditors(ctx context.Context, req *http.Request, additionalEditors []RequestEditorFn) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	for _, r := range additionalEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// ClientWithResponses builds on ClientInterface to offer response payloads
type ClientWithResponses struct {
	ClientInterface
}

// NewClientWithResponses creates a new ClientWithResponses, which wraps
// Client with return type handling
func NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error) {
	client, err := NewClient(server, opts...)
	if err != nil {
		return nil, err
	}
	return &ClientWithResponses{client}, nil
}

// WithBaseURL overrides the baseURL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) error {
		newBaseURL, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		c.Server = newBaseURL.String()
		return nil
	}
}

// ClientWithResponsesInterface is the interface specification for the client with responses above.
type ClientWithResponsesInterface interface {
	// GetRedfishV1SystemsSystemIdWithResponse request
	GetRedfishV1SystemsSystemIdWithResponse(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*GetRedfishV1SystemsSystemIdResponse, error)

	// GetRedfishV1SystemsSystemIdMachineLearningModelWithResponse request
	GetRedfishV1SystemsSystemIdMachineLearningModelWithResponse(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*GetRedfishV1SystemsSystemIdMachineLearningModelResponse, error)
}

type GetRedfishV1SystemsSystemIdResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *System
	JSON401      *Unauthorized
	JSON403      *Forbidden
	JSON404      *NotFound
	JSON405      *MethodNotAllowed
	JSON406      *NotAcceptable
	JSON500      *InternalError
}

// Status returns HTTPResponse.Status
func (r GetRedfishV1SystemsSystemIdResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetRedfishV1SystemsSystemIdResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

type GetRedfishV1SystemsSystemIdMachineLearningModelResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *MachineLearningModel
	JSON401      *Unauthorized
	JSON403      *Forbidden
	JSON404      *NotFound
	JSON405      *MethodNotAllowed
	JSON406      *NotAcceptable
	JSON500      *InternalError
}

// Status returns HTTPResponse.Status
func (r GetRedfishV1SystemsSystemIdMachineLearningModelResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetRedfishV1SystemsSystemIdMachineLearningModelResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// GetRedfishV1SystemsSystemIdWithResponse request returning *GetRedfishV1SystemsSystemIdResponse
func (c *ClientWithResponses) GetRedfishV1SystemsSystemIdWithResponse(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*GetRedfishV1SystemsSystemIdResponse, error) {
	rsp, err := c.GetRedfishV1SystemsSystemId(ctx, systemId, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetRedfishV1SystemsSystemIdResponse(rsp)
}

// GetRedfishV1SystemsSystemIdMachineLearningModelWithResponse request returning *GetRedfishV1SystemsSystemIdMachineLearningModelResponse
func (c *ClientWithResponses) GetRedfishV1SystemsSystemIdMachineLearningModelWithResponse(ctx context.Context, systemId string, reqEditors ...RequestEditorFn) (*GetRedfishV1SystemsSystemIdMachineLearningModelResponse, error) {
	rsp, err := c.GetRedfishV1SystemsSystemIdMachineLearningModel(ctx, systemId, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetRedfishV1SystemsSystemIdMachineLearningModelResponse(rsp)
}

// ParseGetRedfishV1SystemsSystemIdResponse parses an HTTP response from a GetRedfishV1SystemsSystemIdWithResponse call
func ParseGetRedfishV1SystemsSystemIdResponse(rsp *http.Response) (*GetRedfishV1SystemsSystemIdResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetRedfishV1SystemsSystemIdResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest System
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 401:
		var dest Unauthorized
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON401 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 403:
		var dest Forbidden
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON403 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 405:
		var dest MethodNotAllowed
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON405 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 406:
		var dest NotAcceptable
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON406 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 500:
		var dest InternalError
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON500 = &dest

	}

	return response, nil
}

// ParseGetRedfishV1SystemsSystemIdMachineLearningModelResponse parses an HTTP response from a GetRedfishV1SystemsSystemIdMachineLearningModelWithResponse call
func ParseGetRedfishV1SystemsSystemIdMachineLearningModelResponse(rsp *http.Response) (*GetRedfishV1SystemsSystemIdMachineLearningModelResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetRedfishV1SystemsSystemIdMachineLearningModelResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest MachineLearningModel
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 401:
		var dest Unauthorized
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON401 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 403:
		var dest Forbidden
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON403 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 404:
		var dest NotFound
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON404 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 405:
		var dest MethodNotAllowed
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON405 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 406:
		var dest NotAcceptable
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON406 = &dest

	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 500:
		var dest InternalError
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON500 = &dest

	}

	return response, nil
}
