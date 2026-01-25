package markdownx

import (
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
)

// 创建 HTML 清理策略（UGC 模式，允许常见安全标签）
var htmlSanitizer = createSanitizer()

// ToHTML 将 Markdown 转换为 HTML
func ToHTML(content []byte) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(content)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	rawHTML := string(markdown.Render(doc, renderer))

	// 清理 HTML，移除危险标签和属性（XSS 防护）
	sanitizedHTML := htmlSanitizer.Sanitize(rawHTML)

	return wrapTailwindClass(patchMermaidClass(sanitizedHTML))
}

// createSanitizer 创建 HTML 清理策略
func createSanitizer() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// 允许代码块的 class 属性（用于语法高亮）
	regex := regexp.MustCompile(`^(language-[a-zA-Z0-9_-]+|mermaid|[a-zA-Z0-9_\- ]+)$`)
	p.AllowAttrs("class").Matching(regex).OnElements("code", "pre", "span")
	// 允许图片
	p.AllowImages()
	// 允许链接在新标签页打开
	p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`^noopener noreferrer$`)).OnElements("a")
	// 允许 id 属性（用于标题锚点）
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).Globally()
	return p
}

// gomarkdown 会把 mermaid 块转换成 <code class="language-mermaid">，这其实是不正确的，应该是 <code class="mermaid">
func patchMermaidClass(htmlContent string) string {
	return strings.ReplaceAll(htmlContent, "<code class=\"language-mermaid\">", "<code class=\"mermaid\">")
}

// 完全匹配 Html 标签的情况
var fullMatchHtmlTagClassMap = map[string]string{
	"p":   "my-2 mx-2",
	"ol":  "pl-1 list-decimal list-inside",
	"ul":  "pl-4 list-disc",
	"li":  "ml-4 my-2",
	"pre": "my-4",
	// markdown 单行 code 效果（多行另外处理）
	"code": "bg-gray-100 text-orange-600",
	// 使用 left-padding + left-border + bg-color 实现 markdown 引用的效果 :D
	"blockquote": "mt-2 pl-2 py-1 border-l-8 border-green-200 bg-green-100",
	// 斑马表格：奇偶数行不同背景色
	"table": "table-auto border-collapse border border-gray-500",
	"tr":    "odd:bg-white even:bg-gray-100",
	"th":    "border border-gray-500 px-4 py-2",
	"td":    "border border-gray-500 px-4 py-2",
}

// 前缀匹配 Html 标签的情况
var prefixMatchHtmlTagClassMap = map[string]string{
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
	// 前缀匹配的情况
	for tagName, class := range prefixMatchHtmlTagClassMap {
		htmlContent = strings.ReplaceAll(htmlContent, "<"+tagName, "<"+tagName+" class=\""+class+"\"")
	}
	// 完全匹配的情况
	for tagName, class := range fullMatchHtmlTagClassMap {
		htmlContent = strings.ReplaceAll(htmlContent, "<"+tagName+">", "<"+tagName+" class=\""+class+"\">")
	}
	// 对带有 language 标识的 code 标签特殊处理
	re := regexp.MustCompile(`<code class="language-[a-zA-Z]+`)
	return re.ReplaceAllString(htmlContent, "$0 "+codeTagAdditionalClass)
}
