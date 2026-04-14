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

