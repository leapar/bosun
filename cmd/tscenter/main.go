package main

import (
	"bytes"
	_ "expvar"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/facebookgo/httpcontrol"

	version "github.com/leapar/bosun/_version"

	"github.com/leapar/bosun/opentsdb"
	"github.com/leapar/bosun/slog"
	"github.com/leapar/bosun/util"
)

var (
	ciServer = "https://dc-cloud.oneapm.com"
	ddServer = "https://app.datadoghq.com"
	piServer = "http://api.apmsys.com:8000"

	listenAddr = ":10000" //flag.String("l", ":4242", "Listen address.")

	logVerbose  = flag.Bool("v", true, "enable verbose logging")
	flagVersion = flag.Bool("version", false, "Prints the version and exits.")
)

var (
	ciServerInfo *ServerInfo
	ddServerInfo *ServerInfo
	piServerInfo *ServerInfo

	relayPutUrls []string

	tags = opentsdb.TagSet{}
)

type ServerInfo struct {
	intake   string
	series   string
	checkRun string
}

type tsdbrelayHTTPTransport struct {
	UserAgent string
	http.RoundTripper
}

func (t *tsdbrelayHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", t.UserAgent)
	}
	return t.RoundTripper.RoundTrip(req)
}

func init() {
	client := &http.Client{
		Transport: &tsdbrelayHTTPTransport{
			"Tsdbrelay/" + version.ShortVersion(),
			&httpcontrol.Transport{
				RequestTimeout: time.Minute,
			},
		},
	}
	http.DefaultClient = client
}

func main() {
	var err error
	myHost, err = os.Hostname()
	if err != nil || myHost == "" {
		myHost = "tsdbrelay"
	}

	flag.Parse()
	if *flagVersion {
		fmt.Println(version.GetVersionInfo("tsdbrelay"))
		os.Exit(0)
	}

	slog.Infoln(version.GetVersionInfo("tsdbrelay"))
	slog.Infoln("listen on", listenAddr)
	slog.Infoln("relay to ci at", ciServer)
	slog.Infoln("relay to datadog at", ddServer)
	slog.Infoln("relay to pi at", piServer)
	/*
		ciURL := &url.URL{
			Scheme: "http",
			Host:   ciServer,
		}

		u := url.URL{
			Scheme: "http",
			Host:   ciServer,
			Path:   "/api/put",
		}
		ciPutURL = u.String()
		ddURL := &url.URL{
			Scheme: "http",
			Host:   ddServer,
		}
		u = url.URL{
			Scheme: "http",
			Host:   ddServer,
			Path:   "/api/index",
		}

		ciProxy := util.NewSingleHostProxy(ciURL)
		ddProxy := util.NewSingleHostProxy(ddURL)

		http.HandleFunc("/api/put", func(w http.ResponseWriter, r *http.Request) {
			rp.relayPut(w, r, true)
		})

		http.HandleFunc("/api/metadata/put", func(w http.ResponseWriter, r *http.Request) {
			rp.relayMetadata(w, r)
		})

	*/

	ciServerInfo = &ServerInfo{
		intake:   ciServer + "/infrastructure/metrics?license_key=VgAAA1sNB1Ze953TH1YTVV5KXxb55aBVWR9QBQRXSe38fAMASwkKSgNU92f1CAMeUgIYUFA=",
		series:   ciServer + "/infrastructure/metrics?license_key=VgAAA1sNB1Ze953TH1YTVV5KXxb55aBVWR9QBQRXSe38fAMASwkKSgNU92f1CAMeUgIYUFA=",
		checkRun: ciServer + "/infrastructure/metrics?license_key=VgAAA1sNB1Ze953TH1YTVV5KXxb55aBVWR9QBQRXSe38fAMASwkKSgNU92f1CAMeUgIYUFA=",
	}

	ddServerInfo = &ServerInfo{
		intake:   ddServer + "/intake?api_key=e7afaf986f5cc822406cbd5831328462",
		series:   ddServer + "/api/v1/series?api_key=e7afaf986f5cc822406cbd5831328462",
		checkRun: ddServer + "/api/v1/check_run?api_key=e7afaf986f5cc822406cbd5831328462",
	}

	piServerInfo = &ServerInfo{
		intake:   piServer + "/intake",
		series:   piServer + "/api/v1/series",
		checkRun: piServer + "/api/v1/check_run",
	}

	piProxy := util.NewSingleHostProxy(&url.URL{
		Scheme: "http",
		Host:   "api.apmsys.com:8000",
	})

	rp := &relayProxy{
		PIProxy: piProxy,
	}

	http.HandleFunc("/intake/", func(w http.ResponseWriter, r *http.Request) {
		rp.relayPut(w, r, 1, true)
		//w.WriteHeader(200)
		//w.Write([]byte("{\"status\":\"ok2\"}"))
	})
	http.HandleFunc("/api/v1/series/", func(w http.ResponseWriter, r *http.Request) {
		rp.relayPut(w, r, 2, true)
		//w.WriteHeader(200)
		//w.Write([]byte("{\"status\":\"ok2\"}"))
	})
	http.HandleFunc("/api/v1/check_run/", func(w http.ResponseWriter, r *http.Request) {
		rp.relayPut(w, r, 3, true)
		//w.WriteHeader(200)
		//w.Write([]byte("{\"status\":\"ok2\"}"))
	})

	//http.Handle("/", piProxy)

	slog.Fatal(http.ListenAndServe(listenAddr, nil))
}

func verbose(format string, a ...interface{}) {
	if *logVerbose {
		slog.Infof(format, a...)
	} else {
		slog.Info("....")
	}
}

type relayProxy struct {
	PIProxy *httputil.ReverseProxy
}

type passthru struct {
	io.ReadCloser
	buf bytes.Buffer
}

func (p *passthru) Read(b []byte) (int, error) {
	n, err := p.ReadCloser.Read(b)
	p.buf.Write(b[:n])
	return n, err
}

type relayWriter struct {
	http.ResponseWriter
	code int
}

func (rw *relayWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

var (
	relayHeader  = "X-Relayed-From"
	encHeader    = "Content-Encoding"
	typeHeader   = "Content-Type"
	accessHeader = "X-Access-Token"
	myHost       string
)

func (rp *relayProxy) relayPut(responseWriter http.ResponseWriter, r *http.Request, flag int, parse bool) {
	slog.Infoln("relayPut")
	reader := &passthru{ReadCloser: r.Body}
	r.Body = reader

	ioutil.ReadAll(r.Body) /*

		w := &relayWriter{ResponseWriter: responseWriter}

		rp.PIProxy.ServeHTTP(w, r)

		if w.code/100 != 2 {
			verbose("relayPut got status %d", w.code)
			return
		}*/
	verbose("relayed to piServer")
	responseWriter.Write([]byte("{\"status\":\"ok2\"}"))

	// Send to bosun in a separate go routine so we can end the source's request.
	go func() {

		body := bytes.NewBuffer(reader.buf.Bytes())
		//verbose("%d %s", flag, body.String())
		url := ""
		switch flag {
		case 1:
			url = piServerInfo.intake
			break
		case 2:
			url = piServerInfo.series
			break
		case 3:
			url = piServerInfo.checkRun
			break
		}

		if len(r.URL.RawQuery) > 0 {
			url += "?" + r.URL.RawQuery
		}

		req, err := http.NewRequest(r.Method, url, body)
		if err != nil {
			verbose("pi connect error: %v", err)
			return
		}
		for k := range r.Header {
			//verbose(k)
			if access := r.Header.Get(k); access != "" {
				req.Header.Set(k, access)
			}
		}
		//req.Header.Set("Connection", "close")
		//req.Close = true
		/*
			if access := r.Header.Get(accessHeader); access != "" {
				req.Header.Set(accessHeader, access)
			}

			if access := r.Header.Get(typeHeader); access != "" {
				req.Header.Set(typeHeader, access)
			}

			if access := r.Header.Get(encHeader); access != "" {
				req.Header.Set(encHeader, access)
			}
		*/

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			verbose("pi relay error: %v %v", err, body.String())
			return
		}
		resp.Body.Close()
		verbose("pi relay success")
	}()

	go func() {
		//return
		body := bytes.NewBuffer(reader.buf.Bytes())
		url := ""
		switch flag {
		case 1:
			url = ciServerInfo.intake
			break
		case 2:
			url = ciServerInfo.series
			break
		case 3:
			url = ciServerInfo.checkRun
			break
		}

		req, err := http.NewRequest(r.Method, url, body)
		if err != nil {
			verbose("ci connect error: %v", err)
			return
		}
		for k := range r.Header {
			//verbose(k)
			if access := r.Header.Get(k); access != "" {
				req.Header.Set(k, access)
			}
		}
		/*
			if access := r.Header.Get(accessHeader); access != "" {
				req.Header.Set(accessHeader, access)
			}

			if access := r.Header.Get(typeHeader); access != "" {
				req.Header.Set(typeHeader, access)
			}

			if access := r.Header.Get(encHeader); access != "" {
				req.Header.Set(encHeader, access)
			}
		*/
		//req.Header.Set("Connection", "keep-alive")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			verbose("ci relay error: %v %v", err, body.String())
			return
		}
		resp.Body.Close()
		verbose("ci relay success")
	}()

	go func() {
		//return
		body := bytes.NewBuffer(reader.buf.Bytes())
		url := ""
		switch flag {
		case 1:
			url = ddServerInfo.intake
			break
		case 2:
			url = ddServerInfo.series
			break
		case 3:
			url = ddServerInfo.checkRun
			break
		}

		req, err := http.NewRequest(r.Method, url, body)
		if err != nil {
			verbose("datadog connect error: %v", err)
			return
		}

		for k := range r.Header {
			//verbose(k)
			if access := r.Header.Get(k); access != "" {
				req.Header.Set(k, access)
			}
		}

		//req.Header.Set("Connection", "keep-alive")
		/*
			if access := r.Header.Get(accessHeader); access != "" {
				req.Header.Set(accessHeader, access)
			}

			if access := r.Header.Get(typeHeader); access != "" {
				req.Header.Set(typeHeader, access)
			}

			if access := r.Header.Get(encHeader); access != "" {
				req.Header.Set(encHeader, access)
			}
		*/
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			verbose("datadog relay error: %v %s", err, body.String())
			return
		}
		resp.Body.Close()
		verbose("datadog relay success")
	}()

	r.Close = true
}
