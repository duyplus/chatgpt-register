package codexreg

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// gpuProfile A set of real Windows GPU WebGL vendor/renderer strings.
type gpuProfile struct{ vendor, renderer string }

var gpuPool = []gpuProfile{
	{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 (0x00002503) Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 Ti (0x00002182) Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 (0x00002882) Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (Intel)", "ANGLE (Intel, Intel(R) UHD Graphics 630 (0x00003E9B) Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (Intel)", "ANGLE (Intel, Intel(R) Iris(R) Xe Graphics (0x00009A49) Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 6600 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon(TM) Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)"},
}

var screenPool = [][2]int{
	{1920, 1080}, {1536, 864}, {1366, 768}, {1600, 900}, {2560, 1440}, {1440, 900},
}

var corePool = []int{4, 6, 8, 12, 16}
var memPool = []int{8, 16}
var platVerPool = []string{"10.0.0", "15.0.0", "19.0.0"} // Win10 / Win11

// fingerprint Browser fingerprint profile for a single account. Generated deterministically with email as seed.
type fingerprint struct {
	screenW, screenH int
	winW, winH       int
	cores            int
	memory           int
	gpu              gpuProfile
	platformVersion  string
	seed             uint32
}

// newFingerprint Deterministically derives fingerprint parameters from email.
func newFingerprint(email string) *fingerprint {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	seed := h.Sum32()
	r := rand.New(rand.NewSource(int64(seed)))

	sc := screenPool[r.Intn(len(screenPool))]
	// Window size slightly smaller than screen, close to real viewport with taskbar/tab bar.
	winW := sc[0] - 40 - r.Intn(120)
	winH := sc[1] - 120 - r.Intn(120)
	if winW < 1000 {
		winW = 1000
	}
	if winH < 700 {
		winH = 700
	}
	return &fingerprint{
		screenW:         sc[0],
		screenH:         sc[1],
		winW:            winW,
		winH:            winH,
		cores:           corePool[r.Intn(len(corePool))],
		memory:          memPool[r.Intn(len(memPool))],
		gpu:             gpuPool[r.Intn(len(gpuPool))],
		platformVersion: platVerPool[r.Intn(len(platVerPool))],
		seed:            seed,
	}
}

// windowSizeArg For launcher --window-size.
func (f *fingerprint) windowSizeArg() string {
	return fmt.Sprintf("%d,%d", f.winW, f.winH)
}

// detectChromeVersion Reads actual browser kernel version.
// Returns major version (e.g. "131") and full version (e.g. "131.0.6778.86").
func detectChromeVersion(browser *rod.Browser) (major, full string) {
	major, full = "131", "131.0.6778.86" // fallback
	v, err := (proto.BrowserGetVersion{}).Call(browser)
	if err != nil || v == nil {
		return
	}
	p := v.Product // e.g. "HeadlessChrome/131.0.6778.86" or "Chrome/131.0.6778.86"
	if i := strings.LastIndex(p, "/"); i >= 0 && i+1 < len(p) {
		full = p[i+1:]
	}
	if j := strings.Index(full, "."); j > 0 {
		major = full[:j]
	}
	return
}

// applyUserAgent Injects consistent UA and Client Hints based on actual kernel version.
func (f *fingerprint) applyUserAgent(page *rod.Page, browser *rod.Browser, acceptLang string) (major, full string) {
	major, full = detectChromeVersion(browser)
	ua := fmt.Sprintf(
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36",
		major,
	)
	meta := &proto.EmulationUserAgentMetadata{
		Brands: []*proto.EmulationUserAgentBrandVersion{
			{Brand: "Chromium", Version: major},
			{Brand: "Google Chrome", Version: major},
			{Brand: "Not_A Brand", Version: "24"},
		},
		FullVersionList: []*proto.EmulationUserAgentBrandVersion{
			{Brand: "Chromium", Version: full},
			{Brand: "Google Chrome", Version: full},
			{Brand: "Not_A Brand", Version: "24.0.0.0"},
		},
		Platform:        "Windows",
		PlatformVersion: f.platformVersion,
		Architecture:    "x86",
		Bitness:         "64",
		Mobile:          false,
	}
	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent:         ua,
		AcceptLanguage:    acceptLang,
		Platform:          "Win32",
		UserAgentMetadata: meta,
	})
	return
}

// inject Injects fingerprint patch scripts before navigation.
func (f *fingerprint) inject(page *rod.Page) {
	js := fmt.Sprintf(`(function(){
		try {
			var def = function(obj, prop, val){ try{ Object.defineProperty(obj, prop, {get:function(){return val;}, configurable:true}); }catch(e){} };
			def(navigator, 'hardwareConcurrency', %d);
			def(navigator, 'deviceMemory', %d);
			var sw=%d, sh=%d;
			def(screen,'width',sw); def(screen,'height',sh);
			def(screen,'availWidth',sw); def(screen,'availHeight',sh-40);
			def(screen,'colorDepth',24); def(screen,'pixelDepth',24);
			var V=%q, R=%q;
			var patch = function(p){
				if(!p) return;
				var gp = p.getParameter;
				p.getParameter = function(x){
					if(x===37445) return V;   // UNMASKED_VENDOR_WEBGL
					if(x===37446) return R;   // UNMASKED_RENDERER_WEBGL
					if(x===7936)  return V;   // VENDOR
					if(x===7937)  return R;   // RENDERER
					return gp.apply(this, arguments);
				};
			};
			patch(window.WebGLRenderingContext && WebGLRenderingContext.prototype);
			patch(window.WebGL2RenderingContext && WebGL2RenderingContext.prototype);
			var s=(%d)>>>0;
			var rnd = function(){ s=(s*1664525+1013904223)>>>0; return s/4294967296; };
			var noisify = function(canvas){
				try{
					var ctx = canvas.getContext('2d');
					if(!ctx) return;
					var w=canvas.width, h=canvas.height;
					if(!w||!h) return;
					var img = ctx.getImageData(0,0,w,h);
					for(var i=0;i<img.data.length;i+=4){ if(rnd()<0.02){ img.data[i]=img.data[i]^(rnd()<0.5?1:0); } }
					ctx.putImageData(img,0,0);
				}catch(e){}
			};
			var td = HTMLCanvasElement.prototype.toDataURL;
			HTMLCanvasElement.prototype.toDataURL = function(){ noisify(this); return td.apply(this, arguments); };
			var tb = HTMLCanvasElement.prototype.toBlob;
			if(tb){ HTMLCanvasElement.prototype.toBlob = function(){ noisify(this); return tb.apply(this, arguments); }; }
			try{
				var af = AnalyserNode.prototype.getFloatFrequencyData;
				AnalyserNode.prototype.getFloatFrequencyData = function(arr){ af.apply(this, arguments); for(var i=0;i<arr.length;i++){ arr[i]=arr[i]+(rnd()-0.5)*1e-4; } };
			}catch(e){}
		} catch(e){}
		})();`,
		f.cores, f.memory,
		f.screenW, f.screenH,
		f.gpu.vendor, f.gpu.renderer,
		f.seed,
	)
	page.MustEvalOnNewDocument(js)
}
