package platform

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linweiyuan/go-chatgpt-api/api"

	http "github.com/bogdanfinn/fhttp"
)

func ListModels(c *gin.Context) {
	handleGet(c, apiListModels)
}

func RetrieveModel(c *gin.Context) {
	model := c.Param("model")
	handleGet(c, fmt.Sprintf(apiRetrieveModel, model))
}

//goland:noinspection GoUnhandledErrorResult
func CreateCompletions(c *gin.Context) {
	var request CreateCompletionsRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	resp, err := handlePost(c, apiCreateCompletions, data, request.Stream)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	if request.Stream {
		api.HandleConversationResponse(c, resp)
	} else {
		io.Copy(c.Writer, resp.Body)
	}
}

//goland:noinspection GoUnhandledErrorResult
//func CreateChatCompletions(c *gin.Context) {
//	var request ChatCompletionsRequest
//	c.ShouldBindJSON(&request)
//	data, _ := json.Marshal(request)
//	resp, err := handlePost(c, apiCreataeChatCompletions, data, request.Stream)
//	if err != nil {
//		return
//	}
//
//	defer resp.Body.Close()
//	if request.Stream {
//		api.HandleConversationResponse(c, resp)
//	} else {
//		io.Copy(c.Writer, resp.Body)
//	}
//}

func CreateChatCompletions(c *gin.Context) {
	var request ChatCompletionsRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	stream, err := CreateCompletionStream(c, apiCreataeChatCompletions, data, request.Stream)
	if err != nil {
		return
	}

	chanStream := make(chan string, 100)
	go func() {
		defer stream.Close()
		defer close(chanStream)
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				fmt.Println("Stream finished")
				chanStream <- "<!finish>"
				return
			}

			if err != nil {
				fmt.Printf("Stream error: %v\n", err)
				chanStream <- "<!error>"
				return
			}
			if len(response.Choices) == 0 {
				fmt.Println("Stream finished")
				chanStream <- "<!finish>"
				return
			}
			data, err := json.Marshal(response)
			chanStream <- string(data)
			fmt.Printf("Stream response: %v\n", response)
		}
	}()

	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-chanStream; ok {
			if msg == "<!finish>" {
				c.SSEvent("stop", "[DONE]")
				return true
			}
			if msg == "<!error>" {
				c.SSEvent("stop", "[DONE]")
				return true
			}
			c.SSEvent("message", msg)
			fmt.Printf("message: %v\n", msg)

			return true
		}
		return false
	})

}

//goland:noinspection GoUnhandledErrorResult
func CreateEdit(c *gin.Context) {
	var request CreateEditRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	resp, err := handlePost(c, apiCreateEdit, data, false)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

//goland:noinspection GoUnhandledErrorResult
func CreateImage(c *gin.Context) {
	var request CreateImageRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	resp, err := handlePost(c, apiCreateImage, data, false)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

//goland:noinspection GoUnhandledErrorResult
func CreateEmbeddings(c *gin.Context) {
	var request CreateEmbeddingsRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	resp, err := handlePost(c, apiCreateEmbeddings, data, false)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

func ListFiles(c *gin.Context) {
	handleGet(c, apiListFiles)
}

func GetCreditGrants(c *gin.Context) {
	handleGet(c, apiGetCreditGrants)
}

//goland:noinspection GoUnhandledErrorResult
func Login(c *gin.Context) {
	var loginInfo api.LoginInfo
	if err := c.ShouldBindJSON(&loginInfo); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ReturnMessage(api.ParseUserInfoErrorMessage))
		return
	}

	userLogin := UserLogin{
		client: api.NewHttpClient(),
	}

	// hard refresh cookies
	resp, _ := userLogin.client.Get(auth0LogoutUrl)
	defer resp.Body.Close()

	// get authorized url
	authorizedUrl, statusCode, err := userLogin.GetAuthorizedUrl("")
	if err != nil {
		c.AbortWithStatusJSON(statusCode, api.ReturnMessage(err.Error()))
		return
	}

	// get state
	state, _, _ := userLogin.GetState(authorizedUrl)

	// check username
	statusCode, err = userLogin.CheckUsername(state, loginInfo.Username)
	if err != nil {
		c.AbortWithStatusJSON(statusCode, api.ReturnMessage(err.Error()))
		return
	}

	// check password
	code, statusCode, err := userLogin.CheckPassword(state, loginInfo.Username, loginInfo.Password)
	if err != nil {
		c.AbortWithStatusJSON(statusCode, api.ReturnMessage(err.Error()))
		return
	}

	// get access token
	accessToken, statusCode, err := userLogin.GetAccessToken(code)
	if err != nil {
		c.AbortWithStatusJSON(statusCode, api.ReturnMessage(err.Error()))
		return
	}

	// get session key
	var getAccessTokenResponse GetAccessTokenResponse
	json.Unmarshal([]byte(accessToken), &getAccessTokenResponse)
	req, _ := http.NewRequest(http.MethodPost, dashboardLoginUrl, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", api.UserAgent)
	req.Header.Set("Authorization", api.GetAccessToken(getAccessTokenResponse.AccessToken))
	resp, err = userLogin.client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.AbortWithStatusJSON(resp.StatusCode, api.ReturnMessage(getSessionKeyErrorMessage))
		return
	}

	io.Copy(c.Writer, resp.Body)
}

func GetSubscription(c *gin.Context) {
	handleGet(c, apiGetSubscription)
}

func GetApiKeys(c *gin.Context) {
	handleGet(c, apiGetApiKeys)
}

//goland:noinspection GoUnhandledErrorResult
func handleGet(c *gin.Context, url string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	resp, _ := api.Client.Do(req)
	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

func handlePost(c *gin.Context, url string, data []byte, stream bool) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return nil, err
	}

	return resp, nil
}

func CreateCompletionStream(c *gin.Context, url string, data []byte, streamFlag bool) (stream *CompletionStream, err error) {
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	if streamFlag {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, handleErrorResp(resp)
	}

	stream = &CompletionStream{
		streamReader: &streamReader[ChatCompletionStreamResponse]{
			emptyMessagesLimit: 10,
			reader:             bufio.NewReader(resp.Body),
			response:           resp,
			errAccumulator:     newErrorAccumulator(),
			unmarshaler:        &jsonUnmarshaler{},
		},
	}
	return
}

func handleErrorResp(resp *http.Response) error {
	var errRes ErrorResponse
	err := json.NewDecoder(resp.Body).Decode(&errRes)
	if err != nil || errRes.Error == nil {
		reqErr := &RequestError{
			HTTPStatusCode: resp.StatusCode,
			Err:            err,
		}
		if errRes.Error != nil {
			reqErr.Err = errRes.Error
		}
		return reqErr
	}

	errRes.Error.HTTPStatusCode = resp.StatusCode
	return errRes.Error
}

type ErrorResponse struct {
	Error *APIError `json:"error,omitempty"`
}

// APIError provides error information returned by the OpenAI API.
type APIError struct {
	Code           any     `json:"code,omitempty"`
	Message        string  `json:"message"`
	Param          *string `json:"param,omitempty"`
	Type           string  `json:"type"`
	HTTPStatusCode int     `json:"-"`
}

func (e *APIError) Error() string {
	if e.HTTPStatusCode > 0 {
		return fmt.Sprintf("error, status code: %d, message: %s", e.HTTPStatusCode, e.Message)
	}

	return e.Message
}

// RequestError provides informations about generic request errors.
type RequestError struct {
	HTTPStatusCode int
	Err            error
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("error, status code: %d, message: %s", e.HTTPStatusCode, e.Err)
}

type CompletionStream struct {
	*streamReader[ChatCompletionStreamResponse]
}

// CompletionResponse represents a response structure for completion API.
type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   Usage              `json:"usage"`
}

// CompletionChoice represents one of possible completions.
type CompletionChoice struct {
	Text         string        `json:"text"`
	Index        int           `json:"index"`
	FinishReason string        `json:"finish_reason"`
	LogProbs     LogprobResult `json:"logprobs"`
}

// LogprobResult represents logprob result of Choice.
type LogprobResult struct {
	Tokens        []string             `json:"tokens"`
	TokenLogprobs []float32            `json:"token_logprobs"`
	TopLogprobs   []map[string]float32 `json:"top_logprobs"`
	TextOffset    []int                `json:"text_offset"`
}

// Usage Represents the total token usage per request to OpenAI.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
}

type ChatCompletionStreamChoice struct {
	Index        int                             `json:"index"`
	Delta        ChatCompletionStreamChoiceDelta `json:"delta"`
	FinishReason string                          `json:"finish_reason"`
}

type ChatCompletionStreamChoiceDelta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}
