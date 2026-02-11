package openai

import (
	"context"
	"errors"
	"iter"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

var _ model.LLM = &OpenAIModel{}

var (
	ErrNoChoicesInResponse = errors.New("no choices in OpenAI response")
)

// OpenAIModel 实现 model.LLM 接口，支持 thinking 模型
type OpenAIModel struct {
	Client     *openai.Client
	ModelName  string
	SendParams bool // 是否发送 temperature/top_p/max_tokens 等高级参数
}

// NewOpenAIModel 创建 OpenAI 模型
func NewOpenAIModel(modelName string, cfg openai.ClientConfig, sendParams bool) *OpenAIModel {
	client := openai.NewClientWithConfig(cfg)
	return &OpenAIModel{
		Client:     client,
		ModelName:  modelName,
		SendParams: sendParams,
	}
}

// Name 返回模型名称
func (o *OpenAIModel) Name() string {
	return o.ModelName
}

// GenerateContent 实现 model.LLM 接口
func (o *OpenAIModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return o.generateStream(ctx, req)
	}
	return o.generate(ctx, req)
}

// generate 非流式生成
func (o *OpenAIModel) generate(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName, o.SendParams)
		if err != nil {
			yield(nil, err)
			return
		}

		resp, err := o.Client.CreateChatCompletion(ctx, openaiReq)
		if err != nil {
			yield(nil, err)
			return
		}

		llmResp, err := convertChatCompletionResponse(&resp)
		if err != nil {
			yield(nil, err)
			return
		}

		yield(llmResp, nil)
	}
}

// generateStream 流式生成
func (o *OpenAIModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName, o.SendParams)
		if err != nil {
			yield(nil, err)
			return
		}
		openaiReq.Stream = true

		stream, err := o.Client.CreateChatCompletionStream(ctx, openaiReq)
		if err != nil {
			yield(nil, err)
			return
		}
		defer stream.Close()

		o.processStream(stream, yield)
	}
}

// processStream 处理流式响应
func (o *OpenAIModel) processStream(stream *openai.ChatCompletionStream, yield func(*model.LLMResponse, error) bool) {
	aggregatedContent := &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{},
	}
	var finishReason genai.FinishReason
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	toolCallsMap := make(map[int]*toolCallBuilder)
	var textContent string
	var reasoningContent string

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			// 流结束
			break
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// 处理 reasoning_content (thinking 模型)
		if choice.Delta.ReasoningContent != "" {
			reasoningContent += choice.Delta.ReasoningContent
			// 发送 thinking 部分
			part := &genai.Part{Text: choice.Delta.ReasoningContent, Thought: true}
			llmResp := &model.LLMResponse{
				Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
				Partial:      true,
				TurnComplete: false,
			}
			if !yield(llmResp, nil) {
				return
			}
		}

		// 处理普通文本内容
		if choice.Delta.Content != "" {
			textContent += choice.Delta.Content
			part := &genai.Part{Text: choice.Delta.Content}
			llmResp := &model.LLMResponse{
				Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
				Partial:      true,
				TurnComplete: false,
			}
			if !yield(llmResp, nil) {
				return
			}
		}

		// 处理工具调用
		for _, toolCall := range choice.Delta.ToolCalls {
			idx := 0
			if toolCall.Index != nil {
				idx = *toolCall.Index
			}

			if _, exists := toolCallsMap[idx]; !exists {
				toolCallsMap[idx] = &toolCallBuilder{}
			}

			builder := toolCallsMap[idx]
			if toolCall.ID != "" {
				builder.id = toolCall.ID
			}
			if toolCall.Function.Name != "" {
				builder.name = toolCall.Function.Name
			}
			builder.args += toolCall.Function.Arguments
		}

		// 处理结束原因
		if choice.FinishReason != "" {
			finishReason = convertFinishReason(string(choice.FinishReason))
		}

		// 处理 usage
		if chunk.Usage != nil {
			usageMetadata = &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     int32(chunk.Usage.PromptTokens),
				CandidatesTokenCount: int32(chunk.Usage.CompletionTokens),
				TotalTokenCount:      int32(chunk.Usage.TotalTokens),
			}
		}
	}

	// 添加聚合的文本内容
	if textContent != "" {
		aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{Text: textContent})
	}

	// 添加 reasoning content 作为 thought part
	if reasoningContent != "" {
		aggregatedContent.Parts = append([]*genai.Part{{Text: reasoningContent, Thought: true}}, aggregatedContent.Parts...)
	}

	// 添加工具调用
	if len(toolCallsMap) > 0 {
		indices := sortedKeys(toolCallsMap)
		for _, idx := range indices {
			builder := toolCallsMap[idx]
			part := &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   builder.id,
					Name: builder.name,
					Args: parseJSONArgs(builder.args),
				},
			}
			aggregatedContent.Parts = append(aggregatedContent.Parts, part)
		}
	}

	// 发送最终响应
	finalResp := &model.LLMResponse{
		Content:       aggregatedContent,
		UsageMetadata: usageMetadata,
		FinishReason:  finishReason,
		Partial:       false,
		TurnComplete:  true,
	}
	yield(finalResp, nil)
}

// toolCallBuilder 用于聚合流式工具调用
type toolCallBuilder struct {
	id   string
	name string
	args string
}

// sortedKeys 返回排序后的 map keys
func sortedKeys(m map[int]*toolCallBuilder) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单冒泡排序
	for i := 0; i < len(keys)-1; i++ {
		for j := 0; j < len(keys)-i-1; j++ {
			if keys[j] > keys[j+1] {
				keys[j], keys[j+1] = keys[j+1], keys[j]
			}
		}
	}
	return keys
}
