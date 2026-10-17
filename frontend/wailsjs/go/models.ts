export namespace scanner {
	
	export class Result {
	    path: string;
	    sha256: string;
	    status: string;
	    threat?: string;
	    engine: string;
	    scannedAt: string;
	    durationMs: number;
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
	        this.engine = source["engine"];
	        this.scannedAt = source["scannedAt"];
	        this.durationMs = source["durationMs"];
	        this.message = source["message"];
	        this.offline = source["offline"];
	    }
	}

}

