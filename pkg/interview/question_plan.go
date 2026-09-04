package interview

import (
	"regexp"
	"strings"
	"unicode"
)

type QuestionType string

const (
	QuestionTypeConcept    QuestionType = "concept"
	QuestionTypeProject    QuestionType = "project"
	QuestionTypeBehavioral QuestionType = "behavioral"
	QuestionTypeSelfIntro  QuestionType = "self_intro"
	QuestionTypeFollowUp   QuestionType = "follow_up"
	QuestionTypeGeneral    QuestionType = "general"
)

type QuestionPlan struct {
	Type                 QuestionType
	PreviousQuestion     string
	CurrentQueryTerms    []string
	QueryTerms           []string
	ShouldRetrieve       bool
	ProfileBudgetRunes   int
	KnowledgeBudgetRunes int
	KnowledgeLimit       int
	MaxTokens            int
	Instruction          string
}

var asciiTechnicalTermPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.#_-]{1,31}`)

// The list deliberately contains stable technology entities rather than common
// Chinese words. It supplements ASCII extraction without introducing a heavy
// tokenizer dependency into the realtime path.
var knownTechnicalTerms = []string{
	"封装", "继承", "多态", "并发", "事务", "索引", "缓存", "分布式", "微服务",
	"消息队列", "线程池", "协程", "反射", "注解", "垃圾回收", "数据库", "网络协议",
	"数据结构", "算法", "设计模式", "限流", "熔断", "降级", "幂等", "一致性",
	"向量检索", "全文检索", "语义检索", "知识库", "大模型", "语音识别", "注册中心",
	"服务注册", "服务发现", "负载均衡", "网关", "鉴权", "白名单", "请求头", "主从切换",
	"Spring Boot", "Spring Cloud", "CompletableFuture", "HashMap", "MySQL", "Redis",
	"Redisson", "Watchdog", "ThreadPoolExecutor", "LinkedBlockingQueue", "maximumPoolSize",
	"Eureka", "Nacos", "Gateway", "Spring Cloud Gateway", "Feign", "OpenFeign", "Nginx",
	"JWT", "HIS", "PACS", "XXL-JOB", "MinIO", "StripPrefix", "Predicate", "proxy_pass",
	"Kafka", "RabbitMQ", "Elasticsearch", "Docker", "Kubernetes", "Vue", "React",
	"Java", "Python", "Golang", "Go", "C++", "SQL", "RAG", "FTS5", "SQLite",
	"HTTP", "TCP", "TLS", "WebSocket", "SSE", "REST", "RPC", "gRPC", "ASR",
}

func planQuestion(question, previousQuestion string, profileTerms ProfileKeywords, mode string) QuestionPlan {
	question = strings.TrimSpace(question)
	lower := strings.ToLower(question)
	currentTerms := currentQueryTerms(question, profileTerms)
	strongTopicShift := hasStrongTopicShift(question, previousQuestion, currentTerms)
	plan := QuestionPlan{
		Type:                 QuestionTypeGeneral,
		ProfileBudgetRunes:   700,
		KnowledgeBudgetRunes: 1400,
		KnowledgeLimit:       3,
		MaxTokens:            600,
		Instruction:          "直接回答问题，先给结论，再给必要依据；使用自然口语，控制在 180-350 个汉字。",
	}

	switch {
	case containsAny(lower, "自我介绍", "介绍一下你自己", "介绍下你自己", "简单介绍自己"):
		plan.Type = QuestionTypeSelfIntro
		plan.ShouldRetrieve = false
		plan.ProfileBudgetRunes = 2200
		plan.MaxTokens = 760
		plan.Instruction = "这是自我介绍题。围绕目标岗位、核心能力和 1-2 段最相关经历组织成可直接说出口的 60-90 秒回答；不要罗列全部资料。"
	case containsAny(lower, "遇到冲突", "如何处理冲突", "最大的困难", "失败经历", "压力", "优缺点", "职业规划", "为什么离职", "为什么选择我们", "为什么选择这个岗位", "为什么选择本公司", "举一个例子"):
		plan.Type = QuestionTypeBehavioral
		plan.ShouldRetrieve = true
		plan.ProfileBudgetRunes = 1800
		plan.MaxTokens = 700
		plan.Instruction = "这是行为面试题。使用 STAR 结构，说明情境、任务、行动和结果；只引用真实经历，控制在 250-450 个汉字。"
	case !strongTopicShift && isLikelyFollowUp(lower, previousQuestion):
		plan.Type = QuestionTypeFollowUp
		plan.PreviousQuestion = strings.TrimSpace(previousQuestion)
		plan.ShouldRetrieve = true
		plan.ProfileBudgetRunes = 1200
		plan.MaxTokens = 560
		plan.Instruction = "这是对上一轮主题的追问。先结合上一轮问题还原准确技术语境，再直接补充关键原因、实现细节或取舍；不要重复上一轮完整答案，也不要用固定套话解释你如何理解问题，控制在 150-320 个汉字。"
	case isProjectArchitectureQuestion(question, currentTerms) || containsAny(lower, "项目", "经历", "负责", "实现", "技术选型", "难点", "挑战", "线上问题", "怎么做", "如何做") || hasProjectEntityMatch(question, profileTerms):
		plan.Type = QuestionTypeProject
		plan.ShouldRetrieve = true
		plan.ProfileBudgetRunes = 1800
		plan.MaxTokens = 700
		plan.Instruction = "这是项目或经历题。先直接回答当前技术点，再按背景与职责、关键行动、结果、技术取舍组织回答；只有资料明确出现的组件和流程才能说成项目实际方案，控制在 250-450 个汉字。"
	case containsAny(lower, "什么是", "是什么", "有哪些", "区别", "原理", "机制", "特性", "解释一下", "如何理解", "说说"):
		plan.Type = QuestionTypeConcept
		plan.ShouldRetrieve = false
		plan.ProfileBudgetRunes = 0
		plan.KnowledgeBudgetRunes = 0
		plan.KnowledgeLimit = 0
		plan.MaxTokens = 420
		plan.Instruction = "这是通用概念题。按一句定义、2-3 个关键点、一个必要的简短例子回答；除非问题明确要求结合项目，否则不要套用简历经历，控制在 120-250 个汉字。"
	}

	if mode == "general" {
		plan.ShouldRetrieve = false
	}
	if mode == "knowledge_only" {
		plan.ShouldRetrieve = true
		plan.ProfileBudgetRunes = 0
	}

	plan.CurrentQueryTerms = currentTerms
	plan.QueryTerms = buildQueryTerms(question, previousQuestion, plan.Type, profileTerms, currentTerms)
	return plan
}

func isProjectArchitectureQuestion(question string, currentTerms []string) bool {
	return len(currentTerms) > 0 && containsAny(strings.ToLower(question), "项目", "平台", "架构", "调用链路", "业务流程")
}

func hasProjectEntityMatch(question string, profileTerms ProfileKeywords) bool {
	for _, term := range profileTerms.Matches(question) {
		if containsAny(strings.ToLower(term), "项目", "平台", "系统", "服务", "应用") {
			return true
		}
	}
	return false
}

func currentQueryTerms(question string, profileTerms ProfileKeywords) []string {
	terms := extractTechnicalTerms(question)
	terms = appendUniqueTerms(terms, profileTerms.Matches(question)...)
	return terms
}

func buildQueryTerms(question, previousQuestion string, questionType QuestionType, profileTerms ProfileKeywords, currentTerms []string) []string {
	terms := append([]string(nil), currentTerms...)
	if questionType == QuestionTypeFollowUp {
		terms = appendUniqueTerms(terms, extractTechnicalTerms(previousQuestion)...)
		terms = appendUniqueTerms(terms, profileTerms.Matches(previousQuestion)...)
	}
	if len(terms) == 0 {
		fallback := trimQuestionNoise(question)
		if runeLen := len([]rune(fallback)); runeLen >= 2 && runeLen <= 18 {
			terms = append(terms, fallback)
		}
	}
	if len(terms) > 8 {
		terms = terms[:8]
	}
	return terms
}

// hasStrongTopicShift prevents pronouns such as “这个” from binding a new,
// explicitly named project technology to the previous question. A question
// such as “这个医疗预约平台基于 Spring Cloud，主要有哪些组件” must search
// Spring Cloud rather than inherit an earlier Redis/Watchdog topic.
func hasStrongTopicShift(question, previousQuestion string, currentTerms []string) bool {
	if strings.TrimSpace(previousQuestion) == "" || len(currentTerms) == 0 {
		return false
	}
	if !containsAny(strings.ToLower(question), "项目", "平台", "系统", "架构", "链路", "流程", "公司", "实习") {
		return false
	}
	previousTerms := extractTechnicalTerms(previousQuestion)
	if len(previousTerms) == 0 {
		return false
	}
	return !technicalTermsOverlap(currentTerms, previousTerms)
}

func technicalTermsOverlap(left, right []string) bool {
	for _, a := range left {
		a = strings.ToLower(strings.TrimSpace(a))
		for _, b := range right {
			b = strings.ToLower(strings.TrimSpace(b))
			if a != "" && b != "" && (a == b || strings.Contains(a, b) || strings.Contains(b, a)) {
				return true
			}
		}
	}
	return false
}

func extractTechnicalTerms(text string) []string {
	terms := make([]string, 0, 8)
	for _, match := range asciiTechnicalTermPattern.FindAllString(text, -1) {
		if !isGenericASCIIWord(match) {
			terms = appendUniqueTerms(terms, match)
		}
	}
	lower := strings.ToLower(text)
	for _, term := range knownTechnicalTerms {
		if strings.Contains(lower, strings.ToLower(term)) {
			terms = appendUniqueTerms(terms, term)
		}
	}
	return terms
}

func appendUniqueTerms(dst []string, candidates ...string) []string {
	seen := make(map[string]bool, len(dst)+len(candidates))
	for _, term := range dst {
		seen[strings.ToLower(strings.TrimSpace(term))] = true
	}
	for _, term := range candidates {
		term = strings.TrimSpace(term)
		key := strings.ToLower(term)
		if term == "" || seen[key] {
			continue
		}
		seen[key] = true
		dst = append(dst, term)
	}
	return dst
}

func isGenericASCIIWord(term string) bool {
	switch strings.ToLower(term) {
	case "and", "or", "the", "with", "for", "from", "to", "of", "in", "on", "a", "an":
		return true
	default:
		return false
	}
}

func trimQuestionNoise(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "，。！？?!.；;：: ")
	for _, suffix := range []string{"是什么", "有哪些", "怎么做", "如何做", "怎么样", "吗", "呢", "吧"} {
		text = strings.TrimSuffix(text, suffix)
	}
	return strings.TrimSpace(text)
}

func isLikelyFollowUp(question, previousQuestion string) bool {
	if strings.TrimSpace(previousQuestion) == "" {
		return false
	}
	if containsAny(question, "刚才", "上面", "这个", "它", "他", "其", "为什么这样", "还有呢", "具体说说", "展开说说", "再说说", "然后呢", "底层原理", "底层机制") {
		return true
	}
	return len([]rune(question)) <= 12 && !containsAny(question, "什么是", "是什么", "自我介绍")
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func hasSentencePunctuation(text string) bool {
	for _, r := range text {
		if strings.ContainsRune("，。！？；：,.!?;:", r) {
			return true
		}
	}
	return false
}

func alphaNumericRunes(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			count++
		}
	}
	return count
}
