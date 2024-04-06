package markdownx

import (
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

func ToHTML(content []byte) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(content)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return wrapTailwindClass(patchMermaidClass(string(markdown.Render(doc, renderer))))
}

// gomarkdown 会把 mermaid 块转换成 <code class="language-mermaid">，这其实是不正确的，应该是 <code class="mermaid">
func patchMermaidClass(htmlContent string) string {
	return strings.ReplaceAll(htmlContent, "<code class=\"language-mermaid\">", "<code class=\"mermaid\">")
}

var FullMatchHtmlTagClassMap = map[string]string{
	"p":   "my-2 mx-2",
	"ol":  "pl-1 list-decimal list-inside",
	"ul":  "pl-4 list-disc",
	"li":  "ml-4 my-2",
	"pre": "my-4",
	// markdown 单行 code 效果（多行另外处理）
	"code": "bg-gray-100 text-orange-600",
	// 使用 left-padding + left-border + bg-color 实现 markdown 引用的效果 :D
	"blockquote": "pl-2 py-1 border-l-8 border-green-200 bg-green-100",
}

var PrefixMatchHtmlTagClassMap = map[string]string{
	"h1":  "mt-6 mb-4 font-semibold text-3xl",
	"h2":  "mt-6 mb-4 font-semibold text-2xl",
	"h3":  "mt-6 mb-4 font-semibold text-xl",
	"h4":  "mt-6 mb-4 font-semibold text-lg",
	"h5":  "mt-6 mb-4 font-semibold text-base",
	"h6":  "mt-6 mb-4 font-semibold text-base text-gray-600",
	"img": "my-6",
	"a":   "text-blue-500",
}

// 由于 code 标签本身自带 class="language-xxx"，因此不能直接替换，只能补充
var codeTagAdditionalClass = "p-4 rounded-xl"

// wrapTailwindClass 为 markdown 转换成的 html 中的标签添加 tailwind css 类
func wrapTailwindClass(htmlContent string) string {
	// 完全匹配的情况
	for tagName, class := range FullMatchHtmlTagClassMap {
		htmlContent = strings.ReplaceAll(htmlContent, "<"+tagName+">", "<"+tagName+" class=\""+class+"\">")
	}
	// 前缀匹配的情况
	for tagName, class := range PrefixMatchHtmlTagClassMap {
		htmlContent = strings.ReplaceAll(htmlContent, "<"+tagName, "<"+tagName+" class=\""+class+"\"")
	}
	// 对带有 language 标识的 code 标签特殊处理
	re := regexp.MustCompile(`<code class="language-[a-zA-Z]+`)
	return re.ReplaceAllString(htmlContent, "$0 "+codeTagAdditionalClass)
}
