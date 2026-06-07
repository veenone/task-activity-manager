export namespace jira {
	
	export class Transition {
	    id: string;
	    name: string;
	    to: string;
	
	    static createFrom(source: any = {}) {
	        return new Transition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.to = source["to"];
	    }
	}

}

export namespace main {
	
	export class BulkTransitionOptions {
	    currentStatusCounts: Record<string, number>;
	    reachableTargets: string[];
	
	    static createFrom(source: any = {}) {
	        return new BulkTransitionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentStatusCounts = source["currentStatusCounts"];
	        this.reachableTargets = source["reachableTargets"];
	    }
	}
	export class BulkTransitionSkip {
	    testKey: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new BulkTransitionSkip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.testKey = source["testKey"];
	        this.reason = source["reason"];
	    }
	}
	export class BulkTransitionResult {
	    succeeded: string[];
	    skipped: BulkTransitionSkip[];
	    failed: testrepo.BulkFailure[];
	
	    static createFrom(source: any = {}) {
	        return new BulkTransitionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.succeeded = source["succeeded"];
	        this.skipped = this.convertValues(source["skipped"], BulkTransitionSkip);
	        this.failed = this.convertValues(source["failed"], testrepo.BulkFailure);
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
	
	export class HealthInfo {
	    ok: boolean;
	    error: string;
	    dbPath: string;
	    logPath: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.dbPath = source["dbPath"];
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

export namespace syncer {
	
	export class FailedCommit {
	    testKey: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new FailedCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.testKey = source["testKey"];
	        this.error = source["error"];
	    }
	}
	export class Conflict {
	    testKey: string;
	    baseVersion: string;
	    remoteVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new Conflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.testKey = source["testKey"];
	        this.baseVersion = source["baseVersion"];
	        this.remoteVersion = source["remoteVersion"];
	    }
	}
	export class CommitResult {
	    succeeded: string[];
	    conflicted: Conflict[];
	    failed: FailedCommit[];
	
	    static createFrom(source: any = {}) {
	        return new CommitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.succeeded = source["succeeded"];
	        this.conflicted = this.convertValues(source["conflicted"], Conflict);
	        this.failed = this.convertValues(source["failed"], FailedCommit);
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

export namespace testrepo {
	
	export class AllocateResult {
	    added: string[];
	    alreadyMembers: string[];
	
	    static createFrom(source: any = {}) {
	        return new AllocateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.alreadyMembers = source["alreadyMembers"];
	    }
	}
	export class AuditEntry {
	    id: number;
	    occurredAt: string;
	    actor: string;
	    entityType: string;
	    entityKey: string;
	    action: string;
	    field: string;
	    beforeVal: string;
	    afterVal: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.occurredAt = source["occurredAt"];
	        this.actor = source["actor"];
	        this.entityType = source["entityType"];
	        this.entityKey = source["entityKey"];
	        this.action = source["action"];
	        this.field = source["field"];
	        this.beforeVal = source["beforeVal"];
	        this.afterVal = source["afterVal"];
	        this.note = source["note"];
	    }
	}
	export class Bucket {
	    label: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Bucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.count = source["count"];
	    }
	}
	export class BulkEdit {
	    operation: string;
	    field: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new BulkEdit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operation = source["operation"];
	        this.field = source["field"];
	        this.value = source["value"];
	    }
	}
	export class BulkFailure {
	    testKey: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new BulkFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.testKey = source["testKey"];
	        this.error = source["error"];
	    }
	}
	export class BulkEditResult {
	    succeeded: string[];
	    failed: BulkFailure[];
	
	    static createFrom(source: any = {}) {
	        return new BulkEditResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.succeeded = source["succeeded"];
	        this.failed = this.convertValues(source["failed"], BulkFailure);
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
	
	export class Container {
	    key: string;
	    kind: string;
	    summary: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Container(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	    }
	}
	export class ContainerMembership {
	    key: string;
	    kind: string;
	    summary: string;
	    status: string;
	    runStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new ContainerMembership(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.runStatus = source["runStatus"];
	    }
	}
	export class CreateContainerResult {
	    tempKey: string;
	    added: number;
	
	    static createFrom(source: any = {}) {
	        return new CreateContainerResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tempKey = source["tempKey"];
	        this.added = source["added"];
	    }
	}
	export class DeallocateResult {
	    removed: string[];
	    notMembers: string[];
	
	    static createFrom(source: any = {}) {
	        return new DeallocateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removed = source["removed"];
	        this.notMembers = source["notMembers"];
	    }
	}
	export class Folder {
	    id: string;
	    parentId: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parentId = source["parentId"];
	        this.name = source["name"];
	    }
	}
	export class TestCase {
	    key: string;
	    id: string;
	    summary: string;
	    description: string;
	    status: string;
	    priority: string;
	    labels: string[];
	    updated: string;
	    folderId: string;
	
	    static createFrom(source: any = {}) {
	        return new TestCase(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.id = source["id"];
	        this.summary = source["summary"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.labels = source["labels"];
	        this.updated = source["updated"];
	        this.folderId = source["folderId"];
	    }
	}
	export class Page {
	    tests: TestCase[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tests = this.convertValues(source["tests"], TestCase);
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
	export class PendingChange {
	    id: number;
	    entityType: string;
	    entityKey: string;
	    field: string;
	    beforeVal: string;
	    afterVal: string;
	    baseVersion: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new PendingChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.entityType = source["entityType"];
	        this.entityKey = source["entityKey"];
	        this.field = source["field"];
	        this.beforeVal = source["beforeVal"];
	        this.afterVal = source["afterVal"];
	        this.baseVersion = source["baseVersion"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Precondition {
	    key: string;
	    summary: string;
	    type: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new Precondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.summary = source["summary"];
	        this.type = source["type"];
	        this.description = source["description"];
	    }
	}
	export class Query {
	    search: string;
	    status: string;
	    folderId: string;
	    containerKey: string;
	    sortBy: string;
	    desc: boolean;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.search = source["search"];
	        this.status = source["status"];
	        this.folderId = source["folderId"];
	        this.containerKey = source["containerKey"];
	        this.sortBy = source["sortBy"];
	        this.desc = source["desc"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}
	export class SavedView {
	    id: string;
	    name: string;
	    query: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.query = source["query"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class SeedResult {
	    sets: number;
	    plans: number;
	    executions: number;
	    linked: number;
	
	    static createFrom(source: any = {}) {
	        return new SeedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sets = source["sets"];
	        this.plans = source["plans"];
	        this.executions = source["executions"];
	        this.linked = source["linked"];
	    }
	}
	export class Statistics {
	    total: number;
	    pendingChanges: number;
	    executedTests: number;
	    testSets: number;
	    testPlans: number;
	    testExecutions: number;
	    testsInSet: number;
	    testsInPlan: number;
	    byStatus: Bucket[];
	    byPriority: Bucket[];
	    byLabel: Bucket[];
	    byFolder: Bucket[];
	    updatedTrend: Bucket[];
	    byRunStatus: Bucket[];
	
	    static createFrom(source: any = {}) {
	        return new Statistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.pendingChanges = source["pendingChanges"];
	        this.executedTests = source["executedTests"];
	        this.testSets = source["testSets"];
	        this.testPlans = source["testPlans"];
	        this.testExecutions = source["testExecutions"];
	        this.testsInSet = source["testsInSet"];
	        this.testsInPlan = source["testsInPlan"];
	        this.byStatus = this.convertValues(source["byStatus"], Bucket);
	        this.byPriority = this.convertValues(source["byPriority"], Bucket);
	        this.byLabel = this.convertValues(source["byLabel"], Bucket);
	        this.byFolder = this.convertValues(source["byFolder"], Bucket);
	        this.updatedTrend = this.convertValues(source["updatedTrend"], Bucket);
	        this.byRunStatus = this.convertValues(source["byRunStatus"], Bucket);
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
	export class Step {
	    xrayId: string;
	    index: number;
	    action: string;
	    data: string;
	    expected: string;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.xrayId = source["xrayId"];
	        this.index = source["index"];
	        this.action = source["action"];
	        this.data = source["data"];
	        this.expected = source["expected"];
	    }
	}
	export class SyncState {
	    profileId: string;
	    lastSyncedAt: string;
	    testCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.lastSyncedAt = source["lastSyncedAt"];
	        this.testCount = source["testCount"];
	    }
	}
	
	export class TestPlanBoardRow {
	    testKey: string;
	    summary: string;
	    status: string;
	    runStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new TestPlanBoardRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.testKey = source["testKey"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.runStatus = source["runStatus"];
	    }
	}
	export class TestPlanBoard {
	    key: string;
	    summary: string;
	    rows: TestPlanBoardRow[];
	    runCounts: Bucket[];
	
	    static createFrom(source: any = {}) {
	        return new TestPlanBoard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.summary = source["summary"];
	        this.rows = this.convertValues(source["rows"], TestPlanBoardRow);
	        this.runCounts = this.convertValues(source["runCounts"], Bucket);
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

