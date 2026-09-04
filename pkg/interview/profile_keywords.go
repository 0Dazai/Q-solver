package interview

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ProfileKeywords holds rule-extracted terms from the candidate resume.
// Used for query expansion and retrieval re-ranking.
type ProfileKeywords struct {
	Terms []string
}

// profileStopwords are common Chinese words that carry no retrieval value.
var profileStopwords = map[string]bool{
	"的": true, "了": true, "在": true, "是": true, "我": true, "有": true,
	"和": true, "与": true, "及": true, "或": true, "等": true, "也": true,
	"都": true, "就": true, "不": true, "人": true, "一": true, "一个": true,
	"上": true, "下": true, "中": true, "内": true, "外": true, "前": true,
	"后": true, "左": true, "右": true, "大": true, "小": true, "多": true,
	"少": true, "高": true, "低": true, "长": true, "短": true, "新": true,
	"旧": true, "好": true, "坏": true, "快": true, "慢": true, "强": true,
	"弱": true, "重": true, "轻": true, "深": true, "浅": true, "广": true,
	"窄": true, "远": true, "近": true, "他": true, "她": true, "它": true,
	"们": true, "这": true, "那": true, "你": true, "您": true, "其": true,
	"此": true, "该": true, "本": true, "每": true, "各": true, "某": true,
	"什么": true, "怎么": true, "为什么": true, "如何": true, "可以": true,
	"能够": true, "因为": true, "所以": true, "但是": true, "而且": true,
	"如果": true, "虽然": true, "然而": true, "因此": true, "于是": true,
	"并且": true, "以及": true, "或者": true, "还是": true, "不是": true,
	"没有": true, "已经": true, "正在": true, "将要": true, "曾经": true,
	"一直": true, "总是": true, "经常": true, "偶尔": true, "有时": true,
	"自己": true, "本人": true, "个人": true, "公司": true, "项目": true,
	"工作": true, "负责": true, "参与": true, "完成": true, "实现": true,
	"开发": true, "设计": true, "测试": true, "维护": true, "优化": true,
	"使用": true, "利用": true, "采用": true, "应用": true, "通过": true,
	"基于": true, "根据": true, "按照": true, "针对": true, "关于": true,
	"对于": true, "由于": true, "鉴于": true, "考虑到": true, "除此之外": true,
	"另外": true, "此外": true, "同时": true, "随后": true, "最终": true,
	"首先": true, "其次": true, "然后": true, "接着": true, "最后": true,
	"主要": true, "重要": true, "关键": true, "核心": true, "基本": true,
	"基础": true, "根本": true, "主要的": true, "重要的": true, "关键的": true,
}

var profileNamedEntityPattern = regexp.MustCompile(`[\p{Han}A-Za-z0-9+.#_-]{2,18}(项目|平台|系统|服务|应用)`)

// ExtractProfileKeywords extracts bounded technology and project entities.
// Sentence fragments are deliberately rejected: query expansion must never be
// dominated by long resume prose.
func ExtractProfileKeywords(resumeContent string, maxTerms int) ProfileKeywords {
	if maxTerms <= 0 {
		maxTerms = 30
	}
	text := strings.TrimSpace(resumeContent)
	if text == "" {
		return ProfileKeywords{}
	}

	type termScore struct {
		term  string
		score int
	}
	scores := make(map[string]termScore)
	add := func(term string, weight int) {
		term = strings.TrimSpace(strings.Trim(term, "#*-•·：: "))
		if !isValidProfileTerm(term) {
			return
		}
		key := strings.ToLower(term)
		current := scores[key]
		if current.term == "" {
			current.term = term
		}
		current.score += weight
		scores[key] = current
	}
	for _, term := range extractTechnicalTerms(text) {
		add(term, 8)
	}
	for _, entity := range profileNamedEntityPattern.FindAllString(text, -1) {
		add(entity, 6)
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			add(strings.TrimLeft(line, "# "), 5)
		}
		if index := strings.IndexAny(line, "：:"); index > 0 {
			add(line[:index], 3)
		}
	}

	type scoredTerm struct {
		term  string
		score int
	}
	scored := make([]scoredTerm, 0, len(scores))
	for _, item := range scores {
		if item.score <= 0 {
			continue
		}
		scored = append(scored, scoredTerm{term: item.term, score: item.score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].term < scored[j].term
	})

	terms := make([]string, 0, maxTerms)
	for _, st := range scored {
		if len(terms) >= maxTerms {
			break
		}
		terms = append(terms, st.term)
	}
	return ProfileKeywords{Terms: terms}
}

// splitProfileSegments splits resume text on common delimiters.
func splitProfileSegments(text string) []string {
	var segments []string
	var current strings.Builder
	for _, r := range text {
		if isProfileDelimiter(r) {
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

func isProfileDelimiter(r rune) bool {
	switch r {
	case '\n', '\r', '\t', ' ', ',', '，', '.', '。', ';', '；', ':', '：',
		'、', '(', ')', '（', '）', '[', ']', '【', '】', '{', '}', '「', '」',
		'"', '\'', '-', '—', '–', '/', '\\', '|', '&', '*', '#',
		'@', '!', '！', '?', '？', '~', '`', '<', '>', '《', '》':
		return true
	}
	return false
}

func isValidProfileTerm(term string) bool {
	runes := []rune(term)
	if len(runes) < 2 || len(runes) > 32 || hasSentencePunctuation(term) {
		return false
	}
	if strings.ContainsAny(term, "的了是在把将并且以及") && len(runes) > 14 {
		return false
	}
	if profileStopwords[term] {
		return false
	}
	// Reject pure numbers or dates.
	allDigit := true
	for _, r := range runes {
		if !unicode.IsDigit(r) && r != '.' && r != '-' && r != '/' {
			allDigit = false
			break
		}
	}
	if allDigit {
		return false
	}
	// Reject terms that are mostly punctuation or whitespace.
	return alphaNumericRunes(term) >= 2
}

// Matches returns profile terms that overlap with the given text.
// Used for query expansion and re-ranking overlap computation.
func (pk ProfileKeywords) Matches(text string) []string {
	if len(pk.Terms) == 0 || strings.TrimSpace(text) == "" {
		return nil
	}
	var matches []string
	for _, term := range pk.Terms {
		if strings.Contains(text, term) {
			matches = append(matches, term)
		}
	}
	return matches
}
