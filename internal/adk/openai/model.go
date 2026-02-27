package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"slices"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/run-bigpig/jcp/internal/logger"
)

var modelLog = logger.New("openai:model")

var _ model.LLM = &OpenAIModel{}

var (
	ErrNoChoicesInResponse = errors.New("no choices in OpenAI response")
)

// OpenAIModel 实现 model.LLM 接口，支持 thinking 模型
type OpenAIModel struct {
	Client       *openai.Client
	ModelName    string
	SendParams   bool // 是否发送 temperature/top_p/max_tokens 等高级参数
	NoSystemRole bool // 不支持 system role，需降级处理
}

// NewOpenAIModel 创建 OpenAI 模型
func NewOpenAIModel(modelName string, cfg openai.ClientConfig, sendParams bool, noSystemRole bool) *OpenAIModel {
	client := openai.NewClientWithConfig(cfg)
	return &OpenAIModel{
		Client:       client,
		ModelName:    modelName,
		SendParams:   sendParams,
		NoSystemRole: noSystemRole,
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
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName, o.SendParams, o.NoSystemRole)
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
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName, o.SendParams, o.NoSystemRole)
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

	var streamErr error
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = fmt.Errorf("流式读取错误: %w", err)
				modelLog.Warn("流式读取中断: %v", err)
			}
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

	// 添加聚合的文本内容，解析第三方特殊工具调用标记
	if textContent != "" {
		vendorCalls, cleanedText := parseVendorToolCalls(textContent)
		if cleanedText != "" {
			aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{Text: cleanedText})
		}
		// 将第三方工具调用转换为 FunctionCall
		for i, vc := range vendorCalls {
			aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   fmt.Sprintf("vendor_call_%d", i),
					Name: vc.Name,
					Args: vc.Args,
				},
			})
		}
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

	// 流式错误时，yield 错误让上层感知
	if streamErr != nil {
		yield(nil, streamErr)
		return
	}

	// 发送最终聚合响应
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
	slices.Sort(keys)
	return keys
}
