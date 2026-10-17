export namespace main {
	
	export class AppInfo {
	    version: string;
	    productName: string;
	    companyName: string;
	    copyright: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.productName = source["productName"];
	        this.companyName = source["companyName"];
	        this.copyright = source["copyright"];
	    }
	}
	export class Changelog {
	    version: string;
	    heading: string;
	    body: string;
	    source: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Changelog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.heading = source["heading"];
	        this.body = source["body"];
	        this.source = source["source"];
	        this.error = source["error"];
	    }
	}
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    children?: FileEntry[];
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.children = this.convertValues(source["children"], FileEntry);
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
	export class FilePreview {
	    path: string;
	    name: string;
	    ext: string;
	    size: number;
	    sizeText: string;
	    modified: string;
	    isDir: boolean;
	    kind: string;
	    dataURI?: string;
	    columns?: string[];
	    rows?: string[][];
	    sheet?: string;
	    text?: string;
	    note?: string;
	    truncated: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new FilePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.ext = source["ext"];
	        this.size = source["size"];
	        this.sizeText = source["sizeText"];
	        this.modified = source["modified"];
	        this.isDir = source["isDir"];
	        this.kind = source["kind"];
	        this.dataURI = source["dataURI"];
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.sheet = source["sheet"];
	        this.text = source["text"];
	        this.note = source["note"];
	        this.truncated = source["truncated"];
	        this.error = source["error"];
	    }
	}
	export class RecentPath {
	    path: string;
	    name: string;
	    lastUsed: number;
	
	    static createFrom(source: any = {}) {
	        return new RecentPath(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.lastUsed = source["lastUsed"];
	    }
	}
	export class Settings {
	    fontSize: number;
	    theme: string;
	    shell: string;
	    startupDir: string;
	    notifyUpdates: boolean;
	    skippedVersion: string;
	    lastRunVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fontSize = source["fontSize"];
	        this.theme = source["theme"];
	        this.shell = source["shell"];
	        this.startupDir = source["startupDir"];
	        this.notifyUpdates = source["notifyUpdates"];
	        this.skippedVersion = source["skippedVersion"];
	        this.lastRunVersion = source["lastRunVersion"];
	    }
	}
	export class UpdateCheck {
	    currentVersion: string;
	    latestVersion: string;
	    available: boolean;
	    releaseURL: string;
	    publishedAt: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.available = source["available"];
	        this.releaseURL = source["releaseURL"];
	        this.publishedAt = source["publishedAt"];
	        this.error = source["error"];
	    }
	}

}

