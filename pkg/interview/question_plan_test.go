package interview

import (
	"context"
	"strings"
	"testing"

	"Q-Solver/pkg/config"
	"Q-Solver/pkg/knowledge"
)

type countingRetriever struct {
	queries []string
	results []knowledge.SearchResult
}

func (r *countingRetriever) Search(_ context.Context, query string, _ int) ([]knowledge.SearchResult, error) {
	r.queries = append(r.queries, query)
	return r.results, nil
}

func TestConceptQuestionSkipsResumeAndRetrieval(t *testing.T) {
	keywords := ProfileKeywords{Terms: []string{"Java", "智慧课堂项目"}}
	plan := planQuestion("Java 三大特性是什么？", "", keywords, string(knowledge.AnswerModeKnowledgeFirst))
	if plan.Type != QuestionTypeConcept || plan.ShouldRetrieve || plan.ProfileBudgetRunes != 0 {
		t.Fatalf("unexpected concept plan: %+v", plan)
	}
	if !containsFold(plan.QueryTerms, "Java") {
		t.Fatalf("query terms must retain Java: %+v", plan.QueryTerms)
	}
}

func TestProjectQuestionExpandsOnlyMatchedProfileEntities(t *testing.T) {
	keywords := ProfileKeywords{Terms: []string{"Java", "智慧课堂项目", "Paper RAG项目"}}
	plan := planQuestion("智慧课堂项目里怎么处理并发？", "", keywords, string(knowledge.AnswerModeKnowledgeFirst))
	if plan.Type != QuestionTypeProject || !plan.ShouldRetrieve {
		t.Fatalf("unexpected project plan: %+v", plan)
	}
	joined := strings.Join(plan.QueryTerms, " ")
	if !strings.Contains(joined, "智慧课堂项目") || strings.Contains(joined, "Paper RAG") {
		t.Fatalf("query expansion leaked unrelated profile terms: %q", joined)
	}
}

func TestTechnicalChoiceQuestionIsNotBehavioral(t *testing.T) {
	plan := planQuestion("为什么选择 Redis 而不是本地缓存？", "", ProfileKeywords{Terms: []string{"Redis"}}, string(knowledge.AnswerModeKnowledgeFirst))
	if plan.Type == QuestionTypeBehavioral {
		t.Fatalf("technical choice question was misclassified: %+v", plan)
	}
}

func TestTechnicalFollowUpKeepsPreviousQuestionContext(t *testing.T) {
	previous := "Redisson Watchdog 是怎么续期分布式锁的？"
	plan := planQuestion("它的定时续期底层怎么做？", previous, ProfileKeywords{}, string(knowledge.AnswerModeKnowledgeFirst))
	if plan.Type != QuestionTypeFollowUp || plan.PreviousQuestion != previous {
		t.Fatalf("technical follow-up lost its context: %+v", plan)
	}
	input, _ := buildPlannedAnswerInput("它的定时续期底层怎么做？", nil, CandidateProfile{}, plan, knowledge.AnswerModeKnowledgeFirst, nil)
	if !strings.Contains(input, "上一轮面试官问题") || !strings.Contains(input, "Redisson Watchdog") {
		t.Fatalf("previous technical context missing from model input: %q", input)
	}
}

func TestASRPronounMechanismQuestionInheritsWatchdogTopic(t *testing.T) {
	previous := "Redisson Watchdog 是怎么给分布式锁续期的？"
	plan := planQuestion("嗯，他能够续期的机制和底层原理是什么呢？", previous, ProfileKeywords{}, string(knowledge.AnswerModeKnowledgeFirst))
	if plan.Type != QuestionTypeFollowUp || !containsFold(plan.QueryTerms, "Watchdog") {
		t.Fatalf("ASR pronoun follow-up lost previous topic: %+v", plan)
	}
}

func TestNamedPlatformTechnologyDoesNotInheritPreviousRedisTopic(t *testing.T) {
	previous := "Redis Watchdog 的续期时间怎么设置？"
	plan := planQuestion("这个医疗预约平台是基于 Spring Cloud 的，主要用了哪些组件？", previous, ProfileKeywords{}, string(knowledge.AnswerModeKnowledgeFirst))
	if plan.Type != QuestionTypeProject || !plan.ShouldRetrieve {
		t.Fatalf("new platform topic was not routed as a project question: %+v", plan)
	}
	joined := strings.Join(plan.QueryTerms, " ")
	if !strings.Contains(joined, "Spring Cloud") || strings.Contains(joined, "Redis") || strings.Contains(joined, "Watchdog") {
		t.Fatalf("previous topic polluted retrieval query: %q", joined)
	}
	if !containsFold(plan.CurrentQueryTerms, "Spring Cloud") {
		t.Fatalf("current entities missing from plan: %+v", plan.CurrentQueryTerms)
	}
}

func TestCurrentEntityGateRejectsPreviousTopicRecall(t *testing.T) {
	results := []knowledge.SearchResult{
		{Chunk: knowledge.Chunk{ID: "redis", DocumentID: "redis", Content: "Redis Watchdog 自动续期机制"}},
		{Chunk: knowledge.Chunk{ID: "spring", DocumentID: "spring", Content: "Spring Cloud Gateway、Eureka 与 Feign 调用链路"}},
	}
	filtered := FilterRelevantResults(results, []string{"Spring Cloud", "Gateway"})
	if len(filtered) != 1 || filtered[0].DocumentID != "spring" {
		t.Fatalf("current-topic quality gate admitted stale recall: %+v", filtered)
	}
}

func TestProfileKeywordsRejectLongResumeSentences(t *testing.T) {
	resume := "## 智慧课堂项目\n智慧课堂是一个以录播课程为核心的在线学习平台。使用 Java、Redis 和 CompletableFuture 处理并发。"
	keywords := ExtractProfileKeywords(resume, 30)
	joined := strings.Join(keywords.Terms, "|")
	if strings.Contains(joined, "智慧课堂是一个以录播课程为核心") {
		t.Fatalf("long sentence survived keyword extraction: %q", joined)
	}
	for _, expected := range []string{"Java", "Redis", "CompletableFuture", "智慧课堂项目"} {
		if !containsFold(keywords.Terms, expected) {
			t.Fatalf("missing %q in %+v", expected, keywords.Terms)
		}
	}
}

func TestQualityGateRejectsUnrelatedRecallAndDeduplicatesDocuments(t *testing.T) {
	results := []knowledge.SearchResult{
		{Chunk: knowledge.Chunk{ID: "1", DocumentID: "java", Content: "Java 的封装、继承和多态"}},
		{Chunk: knowledge.Chunk{ID: "2", DocumentID: "java", Content: "Java 多态的实现"}},
		{Chunk: knowledge.Chunk{ID: "3", DocumentID: "python", Content: "Python 论文 RAG 项目"}},
	}
	filtered := FilterRelevantResults(results, []string{"Java"})
	if len(filtered) != 1 || filtered[0].DocumentID != "java" {
		t.Fatalf("unexpected quality-gated results: %+v", filtered)
	}
}

func TestPlannedContextUsesBudgetsAndReturnsInjectedCitations(t *testing.T) {
	profile := buildCandidateProfile("## Java项目\n使用 Java 和 Redis 处理并发。\n\n## Python项目\n使用 Python 做论文分析。", nil, 12000)
	plan := planQuestion("Java项目如何处理并发？", "", ExtractProfileKeywords(profile.Context, 30), string(knowledge.AnswerModeKnowledgeFirst))
	results := []knowledge.SearchResult{{Chunk: knowledge.Chunk{ID: "1", DocumentID: "doc", Path: "java.md", Content: strings.Repeat("Java并发资料", 200)}}}
	input, citations := buildPlannedAnswerInput("Java项目如何处理并发？", []string{"我刚才回答了线程池"}, profile, plan, knowledge.AnswerModeKnowledgeFirst, results)
	if len([]rune(input)) > maxRealtimeInputRunes || len(citations) != 1 {
		t.Fatalf("budget or citations invalid: runes=%d citations=%d", len([]rune(input)), len(citations))
	}
	if !strings.Contains(input, "Java") || !strings.Contains(input, "上一轮回答") {
		t.Fatalf("planned context missing required sections: %q", input)
	}
}

func TestManagerRoutesConceptWithoutRAGAndProjectThroughQualityGate(t *testing.T) {
	executor := &recordingAnswerExecutor{inputs: make(chan string, 2)}
	retriever := &countingRetriever{results: []knowledge.SearchResult{
		{Chunk: knowledge.Chunk{ID: "relevant", DocumentID: "java", Path: "java.md", TitlePath: "智慧课堂", Content: "智慧课堂项目使用 Java 和 Redis 处理并发。"}, Score: 2},
		{Chunk: knowledge.Chunk{ID: "noise", DocumentID: "rag", Path: "rag.md", TitlePath: "Paper RAG", Content: "Python 论文资料分析。"}, Score: 1},
	}}
	cfg := configWithKnowledgeFirst()
	manager := NewManager(func() config.Config { return cfg }, func(string, ...any) {}, executor)
	manager.retrievalLog = nil
	manager.ctx = context.Background()
	manager.status = Status{SessionID: "session"}
	manager.knowledgeRetriever = retriever
	manager.retrievalCache = make(map[string][]knowledge.SearchResult)
	manager.candidateProfile = buildCandidateProfile("## 智慧课堂项目\n使用 Java、Redis 处理并发。", nil, 12000)
	manager.profileKeywords = ExtractProfileKeywords(manager.candidateProfile.Context, 40)

	manager.submit(Question{TurnID: "t1", QuestionID: "q1", CorrectedText: "Java 三大特性是什么？", Submitted: true})
	conceptInput := <-executor.inputs
	if len(retriever.queries) != 0 || strings.Contains(conceptInput, "智慧课堂项目") {
		t.Fatalf("concept question used RAG/profile: queries=%+v input=%q", retriever.queries, conceptInput)
	}

	manager.submit(Question{TurnID: "t2", QuestionID: "q2", CorrectedText: "智慧课堂项目如何处理并发？", Submitted: true})
	projectInput := <-executor.inputs
	manager.answers.Wait()
	if len(retriever.queries) != 1 || strings.Contains(retriever.queries[0], "Paper RAG") {
		t.Fatalf("unexpected retrieval query: %+v", retriever.queries)
	}
	if !strings.Contains(projectInput, "Java") || strings.Contains(projectInput, "Python 论文") {
		t.Fatalf("quality gate failed: %q", projectInput)
	}
}

func TestManagerNewPlatformTopicDropsStalePreviousRecall(t *testing.T) {
	executor := &recordingAnswerExecutor{inputs: make(chan string, 1)}
	retriever := &countingRetriever{results: []knowledge.SearchResult{
		{Chunk: knowledge.Chunk{ID: "stale", DocumentID: "redis", Path: "redis.md", TitlePath: "Redis Watchdog", Content: "Redis Watchdog 自动续期和主从切换。"}, Score: 9},
		{Chunk: knowledge.Chunk{ID: "current", DocumentID: "appointment", Path: "appointment.md", TitlePath: "医技预约平台", Content: "Spring Cloud Gateway 通过 Eureka 服务发现，预约服务使用 Feign 调用规则服务。"}, Score: 8},
	}}
	cfg := configWithKnowledgeFirst()
	manager := NewManager(func() config.Config { return cfg }, func(string, ...any) {}, executor)
	manager.retrievalLog = nil
	manager.ctx = context.Background()
	manager.status = Status{SessionID: "session"}
	manager.knowledgeRetriever = retriever
	manager.retrievalCache = make(map[string][]knowledge.SearchResult)
	manager.previousQuestion = "Redis Watchdog 的续期时间怎么设置？"

	manager.submit(Question{TurnID: "t1", QuestionID: "q1", CorrectedText: "这个医疗预约平台基于 Spring Cloud，主要用了哪些组件？", Submitted: true})
	input := <-executor.inputs
	manager.answers.Wait()
	if len(retriever.queries) != 1 || strings.Contains(retriever.queries[0], "Redis") || strings.Contains(retriever.queries[0], "Watchdog") {
		t.Fatalf("stale topic polluted retrieval query: %+v", retriever.queries)
	}
	if !strings.Contains(input, "Eureka") || strings.Contains(input, "自动续期") {
		t.Fatalf("stale recall entered model input: %q", input)
	}
}

func configWithKnowledgeFirst() config.Config {
	cfg := config.Config{}
	cfg.Knowledge.AnswerMode = string(knowledge.AnswerModeKnowledgeFirst)
	return cfg
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(value, expected) {
			return true
		}
	}
	return false
}
