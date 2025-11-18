export namespace main {
	
	export class Group {
	    id: string;
	    name: string;
	    lightIds: string[];
	    on: boolean;
	    brightness: number;
	    temperature: number;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.lightIds = source["lightIds"];
	        this.on = source["on"];
	        this.brightness = source["brightness"];
	        this.temperature = source["temperature"];
	    }
	}
	export class Light {
	    id: string;
	    name: string;
	    on: boolean;
	    brightness: number;
	    temperature: number;
	    productName: string;
	    serialNumber: string;
	
	    static createFrom(source: any = {}) {
	        return new Light(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.on = source["on"];
	        this.brightness = source["brightness"];
	        this.temperature = source["temperature"];
	        this.productName = source["productName"];
	        this.serialNumber = source["serialNumber"];
	    }
	}
	export class Settings {
	    connectionType: string;
	    socketPath: string;
	    apiUrl: string;
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionType = source["connectionType"];
	        this.socketPath = source["socketPath"];
	        this.apiUrl = source["apiUrl"];
	        this.apiKey = source["apiKey"];
	    }
	}
	export class Status {
	    lights: Light[];
	    groups: Group[];
	    onCount: number;
	    offCount: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lights = this.convertValues(source["lights"], Light);
	        this.groups = this.convertValues(source["groups"], Group);
	        this.onCount = source["onCount"];
	        this.offCount = source["offCount"];
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

}

