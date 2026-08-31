export namespace main {
	
	export class ExportParams {
	    imageData: string;
	    cropX: number;
	    cropY: number;
	    cropSize: number;
	    cornerRadius: number;
	    sizes: number[];
	
	    static createFrom(source: any = {}) {
	        return new ExportParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imageData = source["imageData"];
	        this.cropX = source["cropX"];
	        this.cropY = source["cropY"];
	        this.cropSize = source["cropSize"];
	        this.cornerRadius = source["cornerRadius"];
	        this.sizes = source["sizes"];
	    }
	}
	export class ImageInfo {
	    name: string;
	    width: number;
	    height: number;
	    dataUrl: string;
	    detectedRadius: number;
	    cornerDetected: boolean;
	    contentX: number;
	    contentY: number;
	    contentW: number;
	    contentH: number;
	    contentDetected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.dataUrl = source["dataUrl"];
	        this.detectedRadius = source["detectedRadius"];
	        this.cornerDetected = source["cornerDetected"];
	        this.contentX = source["contentX"];
	        this.contentY = source["contentY"];
	        this.contentW = source["contentW"];
	        this.contentH = source["contentH"];
	        this.contentDetected = source["contentDetected"];
	    }
	}

}

