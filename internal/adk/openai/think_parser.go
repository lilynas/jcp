package openai

import "strings"

const (
	thinkOpenTag  = "<think>"
	thinkCloseTag = "</think>"
)

type thinkSegment struct {
	Text    string
	Thought bool
}

// splitThinkTaggedText parses a complete text and maps <think>...</think>
// content into Thought segments.
func splitThinkTaggedText(text string) []thinkSegment {
	if text == "" {
		return nil
	}

	parser := newThinkTagStreamParser()
	segments := parser.Feed(text)
	segments = append(segments, parser.Flush()...)
	return mergeThinkSegments(segments)
}

// thinkTagStreamParser incrementally parses <think>...</think> markers.
type thinkTagStreamParser struct {
	buffer  string
	inThink bool
}

func newThinkTagStreamParser() *thinkTagStreamParser {
	return &thinkTagStreamParser{}
}

// Feed parses incremental chunks and returns deterministically parsed segments.
// Potentially incomplete tag prefixes are kept in internal buffer.
func (p *thinkTagStreamParser) Feed(chunk string) []thinkSegment {
	if chunk == "" {
		return nil
	}

	p.buffer += chunk
	var segments []thinkSegment

	for {
		if p.buffer == "" {
			break
		}

		if p.inThink {
			endIdx := indexFold(p.buffer, thinkCloseTag)
			if endIdx >= 0 {
				if endIdx > 0 {
					segments = append(segments, thinkSegment{
						Text:    p.buffer[:endIdx],
						Thought: true,
					})
				}
				p.buffer = p.buffer[endIdx+len(thinkCloseTag):]
				p.inThink = false
				continue
			}

			emit, keep := splitKeepPossibleTagPrefix(p.buffer, thinkCloseTag)
			if emit != "" {
				segments = append(segments, thinkSegment{
					Text:    emit,
					Thought: true,
				})
			}
			p.buffer = keep
			break
		}

		startIdx := indexFold(p.buffer, thinkOpenTag)
		if startIdx >= 0 {
			if startIdx > 0 {
				segments = append(segments, thinkSegment{
					Text: p.buffer[:startIdx],
				})
			}
			p.buffer = p.buffer[startIdx+len(thinkOpenTag):]
			p.inThink = true
			continue
		}

		emit, keep := splitKeepPossibleTagPrefix(p.buffer, thinkOpenTag)
		if emit != "" {
			segments = append(segments, thinkSegment{
				Text: emit,
			})
		}
		p.buffer = keep
		break
	}

	return mergeThinkSegments(segments)
}

// Flush flushes leftover buffered text, usually called at end-of-stream.
func (p *thinkTagStreamParser) Flush() []thinkSegment {
	if p.buffer == "" {
		return nil
	}

	segment := thinkSegment{
		Text:    p.buffer,
		Thought: p.inThink,
	}
	p.buffer = ""
	p.inThink = false

	if segment.Text == "" {
		return nil
	}
	return []thinkSegment{segment}
}

func splitKeepPossibleTagPrefix(text, tag string) (emit string, keep string) {
	if text == "" || len(tag) <= 1 {
		return text, ""
	}

	maxSuffix := len(tag) - 1
	if maxSuffix > len(text) {
		maxSuffix = len(text)
	}

	for k := maxSuffix; k > 0; k-- {
		if strings.EqualFold(text[len(text)-k:], tag[:k]) {
			return text[:len(text)-k], text[len(text)-k:]
		}
	}

	return text, ""
}

func mergeThinkSegments(segments []thinkSegment) []thinkSegment {
	if len(segments) == 0 {
		return nil
	}

	merged := make([]thinkSegment, 0, len(segments))
	for _, seg := range segments {
		if seg.Text == "" {
			continue
		}

		n := len(merged)
		if n > 0 && merged[n-1].Thought == seg.Thought {
			merged[n-1].Text += seg.Text
			continue
		}

		merged = append(merged, seg)
	}
	return merged
}

func indexFold(s, sep string) int {
	if sep == "" {
		return 0
	}
	return strings.Index(strings.ToLower(s), strings.ToLower(sep))
}

