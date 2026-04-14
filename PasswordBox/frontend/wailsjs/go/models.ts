export namespace main {
	
	export class PasswordEntryVO {
	    id: number;
	    site: string;
	    account: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new PasswordEntryVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.site = source["site"];
	        this.account = source["account"];
	        this.password = source["password"];
	    }
	}

}

export namespace utils {
	
	export class PasswordStrength {
	    score: number;
	    level: string;
	    suggestions: string[];
	
	    static createFrom(source: any = {}) {
	        return new PasswordStrength(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.level = source["level"];
	        this.suggestions = source["suggestions"];
	    }
	}

}

