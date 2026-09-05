export namespace backend {
	
	export class Issue {
	    key: string;
	    id: string;
	    project: string;
	    type: string;
	    summary: string;
	    status: string;
	    assignee: string;
	    reporter: string;
	    priority: string;
	    labels: string[];
	    sprintId: string;
	    sprintName: string;
	    parentKey: string;
	    storyPoints?: number;
	    rank: string;
	    created: string;
	    updated: string;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.id = source["id"];
	        this.project = source["project"];
	        this.type = source["type"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.assignee = source["assignee"];
	        this.reporter = source["reporter"];
	        this.priority = source["priority"];
	        this.labels = source["labels"];
	        this.sprintId = source["sprintId"];
	        this.sprintName = source["sprintName"];
	        this.parentKey = source["parentKey"];
	        this.storyPoints = source["storyPoints"];
	        this.rank = source["rank"];
	        this.created = source["created"];
	        this.updated = source["updated"];
	    }
	}
	export class Link {
	    direction: string;
	    type: string;
	    key: string;
	    summary: string;
	    issueType: string;
	
	    static createFrom(source: any = {}) {
	        return new Link(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.direction = source["direction"];
	        this.type = source["type"];
	        this.key = source["key"];
	        this.summary = source["summary"];
	        this.issueType = source["issueType"];
	    }
	}
	export class IssueDetail {
	    key: string;
	    description: string;
	    links: Link[];
	    fields: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new IssueDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.description = source["description"];
	        this.links = this.convertValues(source["links"], Link);
	        this.fields = source["fields"];
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

export namespace issuerepo {
	
	export class IssuePage {
	    issues: backend.Issue[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new IssuePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issues = this.convertValues(source["issues"], backend.Issue);
	        this.total = source["total"];
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
	export class IssueQuery {
	    text: string;
	    types: string[];
	    sprintId: string;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new IssueQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.types = source["types"];
	        this.sprintId = source["sprintId"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	}
	export class LinkedTest {
	    key: string;
	    summary: string;
	    linkType: string;
	
	    static createFrom(source: any = {}) {
	        return new LinkedTest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.summary = source["summary"];
	        this.linkType = source["linkType"];
	    }
	}
	export class SprintRef {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SprintRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class SyncState {
	    lastSynced: string;
	    lastFull: string;
	    lastError: string;
	    issueCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastSynced = source["lastSynced"];
	        this.lastFull = source["lastFull"];
	        this.lastError = source["lastError"];
	        this.issueCount = source["issueCount"];
	    }
	}

}

export namespace main {
	
	export class Diagnostics {
	    version: string;
	    dbPath: string;
	    sharedPath: string;
	    logPath: string;
	    os: string;
	    arch: string;
	    goVersion: string;
	    schemaVersion: number;
	    profileCount: number;
	    startupError: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.dbPath = source["dbPath"];
	        this.sharedPath = source["sharedPath"];
	        this.logPath = source["logPath"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.goVersion = source["goVersion"];
	        this.schemaVersion = source["schemaVersion"];
	        this.profileCount = source["profileCount"];
	        this.startupError = source["startupError"];
	    }
	}
	export class HealthInfo {
	    ok: boolean;
	    error: string;
	    dbPath: string;
	    sharedPath: string;
	    logPath: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.dbPath = source["dbPath"];
	        this.sharedPath = source["sharedPath"];
	        this.logPath = source["logPath"];
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

export namespace settings {
	
	export class Settings {
	    defaultProfileId: string;
	    theme: string;
	    requirementLinkType: string;
	    showCoverage: boolean;
	    tourSeenVersion: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultProfileId = source["defaultProfileId"];
	        this.theme = source["theme"];
	        this.requirementLinkType = source["requirementLinkType"];
	        this.showCoverage = source["showCoverage"];
	        this.tourSeenVersion = source["tourSeenVersion"];
	    }
	}

}

export namespace syncer {
	
	export class Summary {
	    fetched: number;
	    upserted: number;
	    skipped: number;
	    full: boolean;
	    elapsed: string;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fetched = source["fetched"];
	        this.upserted = source["upserted"];
	        this.skipped = source["skipped"];
	        this.full = source["full"];
	        this.elapsed = source["elapsed"];
	    }
	}

}

