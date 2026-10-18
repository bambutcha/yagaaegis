export namespace history {
	
	export class Entry {
	    path: string;
	    status: string;
	    threat?: string;
	    message?: string;
	    isDir: boolean;
	    hitCount?: number;
	    durationMs: number;
	    scannedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	        this.threat = source["threat"];
	        this.message = source["message"];
	        this.isDir = source["isDir"];
	        this.hitCount = source["hitCount"];
	        this.durationMs = source["durationMs"];
	        this.scannedAt = source["scannedAt"];
	    }
	}

}

export namespace scanner {
	
	export class Hit {
	    path: string;
	    threat: string;
	    kind?: string;
	
	    static createFrom(source: any = {}) {
	        return new Hit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.threat = source["threat"];
	        this.kind = source["kind"];
	    }
	}
	export class Result {
	    path: string;
	    sha256?: string;
	    status: string;
	    threat?: string;
	    hits?: Hit[];
	    isDir: boolean;
	    engine: string;
	    scannedAt: string;
	    durationMs: number;
	    filesScanned?: number;
	    infectedCount?: number;
	    message?: string;
	    offline: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.sha256 = source["sha256"];
	        this.status = source["status"];
	        this.threat = source["threat"];
	        this.hits = this.convertValues(source["hits"], Hit);
	        this.isDir = source["isDir"];
	        this.engine = source["engine"];
	        this.scannedAt = source["scannedAt"];
	        this.durationMs = source["durationMs"];
	        this.filesScanned = source["filesScanned"];
	        this.infectedCount = source["infectedCount"];
	        this.message = source["message"];
	        this.offline = source["offline"];
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

