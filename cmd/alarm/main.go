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
	"encoding/json"
	"strings"

	"github.com/hashicorp/consul/api"
)

var (
	alertServer = "http://127.0.0.1:8181"
	bosunServer = "http://127.0.0.1:8010"
	listenAddr = ":10000" //flag.String("l", ":4242", "Listen address.")
	logVerbose  = flag.Bool("v", true, "enable verbose logging")
	flagVersion = flag.Bool("version", false, "Prints the version and exits.")
)

var (
	relayPutUrls []string
	tags = opentsdb.TagSet{}
)

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
			"alarm/" + version.ShortVersion(),
			&httpcontrol.Transport{
				RequestTimeout: time.Minute,
			},
		},
	}
	http.DefaultClient = client
}

func main() {

	// Get a new client
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// Get a handle to the KV API
	kv := client.KV()


	client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name:    "testing",
		Tags:    []string{"foo", "bar"},
		Port:    8080,
		Address: "127.0.0.1",
	})

	services, meta, err := client.Catalog().Service("testing","",nil)
	if err != nil {
		fmt.Println(err)
	}

	if meta.LastIndex == 0 {
		fmt.Println("Bad: %v", meta)
	}

	if len(services) == 0 {
		fmt.Println("Bad: %v", services)
	}

	if services[0].Datacenter != "dc1" {
		fmt.Println("Bad datacenter: %v", services[0])
	}
	// PUT a new KV pair
	p := &api.KVPair{Key: "foo", Value: []byte("test")}
	_, err = kv.Put(p, nil)
	if err != nil {
		panic(err)
	}

	// Lookup the pair
	pair, _, err := kv.Get("foo", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("KV: %v", pair)

	flag.Parse()
	if *flagVersion {
		fmt.Println(version.GetVersionInfo("tsdbrelay"))
		os.Exit(0)
	}

	slog.Infoln(version.GetVersionInfo("tsdbrelay"))
	slog.Infoln("listen on", listenAddr)

	piProxy := util.NewSingleHostProxy(&url.URL{
		Scheme: "http",
		Host:   alertServer,
	})

	rp := &relayProxy{
		PIProxy: piProxy,
	}

	http.HandleFunc("/close", func(w http.ResponseWriter, r *http.Request) {
		//http://127.0.0.1:10000/api/alert/34c7f36e-6fde-4fd3-952e-6bd8ae8bad67/status
		/*
		{"status":"closed","text":"status change via console by leapar"}
		*/
		rp.closeAlert(w, r, 1, true)
	})

	http.Handle("/", piProxy)
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


type RawData struct {
	RawData map[string]interface{} `json:"rawData"`
}
type Alert struct {
	Alert RawData `json:"alert"`
}
func (rp *relayProxy) closeAlert(responseWriter http.ResponseWriter, r *http.Request, flag int, parse bool) {
	fmt.Println(r.FormValue("id"))

	alertId := r.FormValue("id")

	go func() {
		req, err := http.NewRequest("PUT",
			alertServer + "/api/alert/"+alertId+"/status",
			strings.NewReader("{\"status\":\"closed\",\"text\":\"status change via console by bosun\"}"))
		if err != nil {
			verbose("pi connect error: %v", err)
			return
		}
		req.Header.Set("Content-Type","application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			//verbose("pi relay error: %v %v", err, body.String())
			return
		}
		resp.Body.Close()
	}()


	go func() {
		//http://localhost:8080/alert/1711c57e-5c6a-4c39-881b-9d8d174feafe
		req, err := http.NewRequest("GET", alertServer + "/api/alert/9c1ea8e9-fb2d-4422-8f95-5eceb5b02015", nil)
		if err != nil {
			verbose("pi connect error: %v", err)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			//verbose("pi relay error: %v %v", err, body.String())
			return
		}
		alert := Alert{}
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&alert); err != nil {
			return
		}
		resp.Body.Close()

		key, ok := alert.Alert.RawData["key"].(string)
		fmt.Println(key)
		if key == "" || ok{
			return
		}

		req, err = http.NewRequest("POST",
			bosunServer + "/api/action",
			strings.NewReader(`{"Type":"forceClose","Message":"","Keys":["`+key+`"],"Notify":false}`))
		if err != nil {
			verbose("pi connect error: %v", err)
			return
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			//verbose("pi relay error: %v %v", err, body.String())
			return
		}
		resp.Body.Close()





	}()

	//fmt.Println(alert.Alert.RawData["key"])
	/*
		data := struct {
		Config  string
		Message string
		User    string
		Other   []string
	}{}
	if data.User != "" && !userCanOverwriteUsername(r) {
		http.Error(w, "Not Authorized to set User", 400)
		return nil, nil
	} else if data.User == "" {
		data.User = getUsername(r)
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	*/


	//http://127.0.0.1:10000/api/alert/34c7f36e-6fde-4fd3-952e-6bd8ae8bad67/status
	/*
	{"status":"closed","text":"status change via console by leapar"}
	*/


	/*
	http://127.0.0.1:8010/api/action

{"Type":"forceClose","Message":"","Keys":["system.mem.free{host=paas-177}"],"Notify":false}



	*/

	reader := &passthru{ReadCloser: r.Body}
	r.Body = reader
	ioutil.ReadAll(r.Body)
	responseWriter.Write([]byte("{\"status\":\"ok2\"}"))

	r.Close = true
}


