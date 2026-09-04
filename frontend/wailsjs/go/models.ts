export namespace config {
	
	export class AnswerModelConfig {
	    provider?: string;
	    model?: string;
	    apiKey?: string;
	    apiKeySet?: boolean;
	    baseURL?: string;
	    protocol?: string;
	    thinkingMode?: string;
	    reasoningLevel?: string;
	    disableResponseStorage?: boolean;
	    maxTokens?: number;
	    temperature?: number;
	    systemPrompt?: string;
	
	    static createFrom(source: any = {}) {
	        return new AnswerModelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.apiKey = source["apiKey"];
	        this.apiKeySet = source["apiKeySet"];
	        this.baseURL = source["baseURL"];
	        this.protocol = source["protocol"];
	        this.thinkingMode = source["thinkingMode"];
	        this.reasoningLevel = source["reasoningLevel"];
	        this.disableResponseStorage = source["disableResponseStorage"];
	        this.maxTokens = source["maxTokens"];
	        this.temperature = source["temperature"];
	        this.systemPrompt = source["systemPrompt"];
	    }
	}
	export class KnowledgeConfig {
	    enabled: boolean;
	    answerMode?: string;
	    paths?: string[];
	    cloudEnabled?: boolean;
	    cloudProvider?: string;
	    cloudEndpoint?: string;
	    collection?: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.answerMode = source["answerMode"];
	        this.paths = source["paths"];
	        this.cloudEnabled = source["cloudEnabled"];
	        this.cloudProvider = source["cloudProvider"];
	        this.cloudEndpoint = source["cloudEndpoint"];
	        this.collection = source["collection"];
	    }
	}
	export class TranscriptionConfig {
	    engine?: string;
	    apiKey?: string;
	    apiKeySet?: boolean;
	    model?: string;
	    endpoint?: string;
	    region?: string;
	    language?: string;
	    hotwords?: string[];
	    contextPhrases?: string[];
	    vocabularyId?: string;
	    sentenceWaitMs?: number;
	    autoSubmit?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TranscriptionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.apiKey = source["apiKey"];
	        this.apiKeySet = source["apiKeySet"];
	        this.model = source["model"];
	        this.endpoint = source["endpoint"];
	        this.region = source["region"];
	        this.language = source["language"];
	        this.hotwords = source["hotwords"];
	        this.contextPhrases = source["contextPhrases"];
	        this.vocabularyId = source["vocabularyId"];
	        this.sentenceWaitMs = source["sentenceWaitMs"];
	        this.autoSubmit = source["autoSubmit"];
	    }
	}
	export class Config {
	    apiKey?: string;
	    provider?: string;
	    baseURL?: string;
	    model?: string;
	    prompt?: string;
	    domainId?: string;
	    opacity?: number;
	    noCompression?: boolean;
	    compressionQuality?: number;
	    sharpening?: number;
	    grayscale?: boolean;
	    keepContext?: boolean;
	    interruptThinking?: boolean;
	    screenshotMode?: string;
	    resumePath?: string;
	    resumeContent?: string;
	    shortcuts?: Record<string, shortcut.KeyBinding>;
	    assistantModel?: string;
	    windowWidth?: number;
	    windowHeight?: number;
	    theme?: string;
	    workMode?: string;
	    writtenModel?: AnswerModelConfig;
	    interviewModel?: AnswerModelConfig;
	    transcription?: TranscriptionConfig;
	    knowledge?: KnowledgeConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.provider = source["provider"];
	        this.baseURL = source["baseURL"];
	        this.model = source["model"];
	        this.prompt = source["prompt"];
	        this.domainId = source["domainId"];
	        this.opacity = source["opacity"];
	        this.noCompression = source["noCompression"];
	        this.compressionQuality = source["compressionQuality"];
	        this.sharpening = source["sharpening"];
	        this.grayscale = source["grayscale"];
	        this.keepContext = source["keepContext"];
	        this.interruptThinking = source["interruptThinking"];
	        this.screenshotMode = source["screenshotMode"];
	        this.resumePath = source["resumePath"];
	        this.resumeContent = source["resumeContent"];
	        this.shortcuts = this.convertValues(source["shortcuts"], shortcut.KeyBinding, true);
	        this.assistantModel = source["assistantModel"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.theme = source["theme"];
	        this.workMode = source["workMode"];
	        this.writtenModel = this.convertValues(source["writtenModel"], AnswerModelConfig);
	        this.interviewModel = this.convertValues(source["interviewModel"], AnswerModelConfig);
	        this.transcription = this.convertValues(source["transcription"], TranscriptionConfig);
	        this.knowledge = this.convertValues(source["knowledge"], KnowledgeConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace domain {
	
	export class DomainItem {
	    id: string;
	    label: string;
	    icon: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new DomainItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	    }
	}
	export class Category {
	    id: string;
	    label: string;
	    items: DomainItem[];
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.items = this.convertValues(source["items"], DomainItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace interview {
	
	export class Status {
	    sessionId?: string;
	    eventSequence?: number;
	    running: boolean;
	    systemAudio: string;
	    microphone: boolean;
	    asr: string;
	    engine?: string;
	    droppedPackets: number;
	    queuedPackets: number;
	    partial?: string;
	    candidate?: string;
	    submitted?: string;
	    suppression?: string;
	    answering: boolean;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.eventSequence = source["eventSequence"];
	        this.running = source["running"];
	        this.systemAudio = source["systemAudio"];
	        this.microphone = source["microphone"];
	        this.asr = source["asr"];
	        this.engine = source["engine"];
	        this.droppedPackets = source["droppedPackets"];
	        this.queuedPackets = source["queuedPackets"];
	        this.partial = source["partial"];
	        this.candidate = source["candidate"];
	        this.submitted = source["submitted"];
	        this.suppression = source["suppression"];
	        this.answering = source["answering"];
	        this.lastError = source["lastError"];
	    }
	}

}

export namespace interviewhistory {
	
	export class Message {
	    id: string;
	    sessionId: string;
	    turnId: string;
	    questionId?: string;
	    role: string;
	    content: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.turnId = source["turnId"];
	        this.questionId = source["questionId"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Session {
	    id: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    endedAt?: any;
	    resumePath?: string;
	    model?: string;
	    answerMode?: string;
	    markdownPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.endedAt = this.convertValues(source["endedAt"], null);
	        this.resumePath = source["resumePath"];
	        this.model = source["model"];
	        this.answerMode = source["answerMode"];
	        this.markdownPath = source["markdownPath"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace knowledge {
	
	export class Document {
	    id: string;
	    path: string;
	    contentHash: string;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Document(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.contentHash = source["contentHash"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SearchResult {
	    id: string;
	    documentId: string;
	    path: string;
	    titlePath: string;
	    content: string;
	    contentHash: string;
	    ordinal: number;
	    // Go type: time
	    updatedAt: any;
	    source: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.documentId = source["documentId"];
	        this.path = source["path"];
	        this.titlePath = source["titlePath"];
	        this.content = source["content"];
	        this.contentHash = source["contentHash"];
	        this.ordinal = source["ordinal"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.source = source["source"];
	        this.score = source["score"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace screen {
	
	export class PreviewResult {
	    imgBytes: number[];
	    base64: string;
	    size: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imgBytes = source["imgBytes"];
	        this.base64 = source["base64"];
	        this.size = source["size"];
	    }
	}

}

export namespace shortcut {
	
	export class KeyBinding {
	    vkCode: string;
	    keyName: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vkCode = source["vkCode"];
	        this.keyName = source["keyName"];
	    }
	}

}

