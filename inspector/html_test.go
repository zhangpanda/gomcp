package inspector

import (
	"regexp"
	"strings"
	"testing"
)

// BUG I1: the original inspector HTML built its sidebar, tool details,
// and result panes via string concatenation into innerHTML, injecting
// tool names, descriptions, and URIs as markup. A tool registered
// from an OpenAPI operationId or a gRPC service — either of which
// can originate in third-party source — could inject JavaScript into
// the dev UI. This test pins a few properties of the shipped HTML
// that indicate the safe pattern is still in place.
func TestInspectorHTML_NoUnsafeInnerHTMLConcat(t *testing.T) {
	// No concatenation of dynamic values into innerHTML. The fixed
	// renderer uses DOM APIs (textContent / setAttribute) exclusively,
	// so occurrences of '.innerHTML' should be limited to the
	// assignments that clear containers — none should concatenate.
	//
	// We conservatively fail the test if we spot any pattern like
	//   '.innerHTML = ... + ... '
	// or
	//   '.innerHTML += '
	// because both re-introduce the injection surface.
	badPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\.innerHTML\s*=\s*[^;]*\+`),
		regexp.MustCompile(`\.innerHTML\s*\+=`),
	}
	for _, re := range badPatterns {
		if loc := re.FindStringIndex(inspectorHTML); loc != nil {
			// show a small window of context
			start := loc[0] - 30
			if start < 0 {
				start = 0
			}
			end := loc[1] + 40
			if end > len(inspectorHTML) {
				end = len(inspectorHTML)
			}
			t.Fatalf("inspector HTML contains unsafe innerHTML concatenation: %q",
				inspectorHTML[start:end])
		}
	}

	// And the onclick-string concatenation that existed in the
	// original (onclick="callTool('" + t.name + "')") must not come
	// back — we route clicks via addEventListener instead.
	if strings.Contains(inspectorHTML, `onclick="callTool(`) ||
		strings.Contains(inspectorHTML, `onclick="readResource(`) ||
		strings.Contains(inspectorHTML, `onclick="getPrompt(`) ||
		strings.Contains(inspectorHTML, `onclick="showTool(`) {
		t.Fatal("inspector HTML uses inline onclick=\"...(value)\" attribute — XSS regressed")
	}
}
