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

