export namespace cases {
	
	export class Case {
	    id: number;
	    key: string;
	    state: string;
	    notes: string;
	    openedAt: string;
	    assessed: number;
	    affected: number;
	
	    static createFrom(source: any = {}) {
	        return new Case(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.key = source["key"];
	        this.state = source["state"];
	        this.notes = source["notes"];
	        this.openedAt = source["openedAt"];
	        this.assessed = source["assessed"];
	        this.affected = source["affected"];
	    }
	}
	export class Project {
	    key: string;
	    verdict: string;
	    reason: string;
	    assessedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.verdict = source["verdict"];
	        this.reason = source["reason"];
	        this.assessedAt = source["assessedAt"];
	    }
	}
	export class Detail {
	    id: number;
	    key: string;
	    state: string;
	    notes: string;
	    openedAt: string;
	    assessed: number;
	    affected: number;
	    projects: Project[];
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.key = source["key"];
	        this.state = source["state"];
	        this.notes = source["notes"];
	        this.openedAt = source["openedAt"];
	        this.assessed = source["assessed"];
	        this.affected = source["affected"];
	        this.projects = this.convertValues(source["projects"], Project);
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
	export class Group {
	    id: number;
	    name: string;
	    projects: string[];
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.projects = source["projects"];
	    }
	}

}

export namespace evidence {
	
	export class Item {
	    id: number;
	    project: string;
	    kind: string;
	    ref: string;
	    detail: string;
	    state: string;
	    weak: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project = source["project"];
	        this.kind = source["kind"];
	        this.ref = source["ref"];
	        this.detail = source["detail"];
	        this.state = source["state"];
	        this.weak = source["weak"];
	    }
	}

}

export namespace jira {
	
	export class Project {
	    key: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	    }
	}
	export class RawFilter {
	    id: string;
	    name: string;
	    jql: string;
	    // Go type: struct { Name string "json:\"name\"" }
	    owner: any;
	
	    static createFrom(source: any = {}) {
	        return new RawFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.jql = source["jql"];
	        this.owner = this.convertValues(source["owner"], Object);
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

export namespace profile {
	
	export class Profile {
	    id: string;
	    name: string;
	    jiraUrl: string;
	    projectKey: string;
	    scopeJql: string;
	    crossProjectSources: string;
	    bugIssueType: string;
	    bugProjectMode: string;
	    bugProjectKey: string;
	    caCert: string;
	    allowUntrustedTls: boolean;
	    backend: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.jiraUrl = source["jiraUrl"];
	        this.projectKey = source["projectKey"];
	        this.scopeJql = source["scopeJql"];
	        this.crossProjectSources = source["crossProjectSources"];
	        this.bugIssueType = source["bugIssueType"];
	        this.bugProjectMode = source["bugProjectMode"];
	        this.bugProjectKey = source["bugProjectKey"];
	        this.caCert = source["caCert"];
	        this.allowUntrustedTls = source["allowUntrustedTls"];
	        this.backend = source["backend"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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

export namespace query {
	
	export class Link {
	    key: string;
	    type: string;
	    words: string;
	
	    static createFrom(source: any = {}) {
	        return new Link(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.type = source["type"];
	        this.words = source["words"];
	    }
	}
	export class Issue {
	    key: string;
	    project: string;
	    projectName: string;
	    type: string;
	    summary: string;
	    status: string;
	    priority: string;
	    updated: string;
	    statusCategory: string;
	    created: string;
	    resolved: string;
	    links: Link[];
	    bases: string[];
	    clientVisibility: string;
	    clientImpact: string;
	    matched: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.project = source["project"];
	        this.projectName = source["projectName"];
	        this.type = source["type"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.updated = source["updated"];
	        this.statusCategory = source["statusCategory"];
	        this.created = source["created"];
	        this.resolved = source["resolved"];
	        this.links = this.convertValues(source["links"], Link);
	        this.bases = source["bases"];
	        this.clientVisibility = source["clientVisibility"];
	        this.clientImpact = source["clientImpact"];
	        this.matched = source["matched"];
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
	
	export class Set {
	    keys: string[];
	    projects: string[];
	    outOfStep: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Set(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys = source["keys"];
	        this.projects = source["projects"];
	        this.outOfStep = source["outOfStep"];
	    }
	}
	export class Result {
	    total: number;
	    issues: Issue[];
	    truncated: boolean;
	    sets: Set[];
	    linkError: string;
	    linksTruncated: boolean;
	    needsConfirm: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.issues = this.convertValues(source["issues"], Issue);
	        this.truncated = source["truncated"];
	        this.sets = this.convertValues(source["sets"], Set);
	        this.linkError = source["linkError"];
	        this.linksTruncated = source["linksTruncated"];
	        this.needsConfirm = source["needsConfirm"];
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

export namespace saved {
	
	export class Fields {
	    productBase: string;
	    clientVisibility: string;
	    clientImpact: string;
	
	    static createFrom(source: any = {}) {
	        return new Fields(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.productBase = source["productBase"];
	        this.clientVisibility = source["clientVisibility"];
	        this.clientImpact = source["clientImpact"];
	    }
	}
	export class Query {
	    id: number;
	    name: string;
	    jql: string;
	    runAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.jql = source["jql"];
	        this.runAt = source["runAt"];
	    }
	}
	export class Run {
	    result: query.Result;
	    runAt: string;
	    added: string[];
	    dropped: string[];
	
	    static createFrom(source: any = {}) {
	        return new Run(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.result = this.convertValues(source["result"], query.Result);
	        this.runAt = source["runAt"];
	        this.added = source["added"];
	        this.dropped = source["dropped"];
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

