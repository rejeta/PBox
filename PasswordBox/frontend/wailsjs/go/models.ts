export namespace main {
	
	export class EntryVO {
	    id: number;
	    title: string;
	    url: string;
	    username: string;
	    password: string;
	    note: string;
	    isFavorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EntryVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.note = source["note"];
	        this.isFavorite = source["isFavorite"];
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

