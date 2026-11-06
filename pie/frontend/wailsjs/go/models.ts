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
	    type: string;
	    summary: string;
	    status: string;
	    priority: string;
	    updated: string;
	    statusCategory: string;
	    created: string;
	    resolved: string;
	    links: Link[];
	    matched: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.project = source["project"];
	        this.type = source["type"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.updated = source["updated"];
	        this.statusCategory = source["statusCategory"];
	        this.created = source["created"];
	        this.resolved = source["resolved"];
	        this.links = this.convertValues(source["links"], Link);
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

