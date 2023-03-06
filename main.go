package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"io"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/scheme"

	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	cacert  = flag.String("cacert", "", "verify certificates of TLS-enabled secure servers using this CA bundle (v3)")
	cert    = flag.String("cert", "", "identify secure client using this TLS certificate file (v3)")
	keyfile = flag.String("key", "", "identify secure client using this TLS key file (v3)")
	mu      sync.Mutex
)
var separator = "/"
var host = "192.168.50.50:2379"
var client *clientv3.Client

func main() {
	flag.CommandLine.Parse(os.Args[1:])
	http.HandleFunc("/v3/connect", connect)
	http.HandleFunc("/v3/get", get)
	http.HandleFunc("/v3/getpath", getPath)
	wd, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	rootPath := filepath.Dir(wd)
	log.Println(http.Dir(rootPath + "/assets"))
	http.Handle("/", http.FileServer(http.Dir(rootPath+"/assets"))) // view static directory
	addr := "0.0.0.0:8080"
	log.Printf("listening on %s\n", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func connect(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	newClient(host)
	log.Println(r.Method, "v3", "connect success.")
	info := getInfo()
	b, _ := json.Marshal(map[string]interface{}{"status": "running", "info": info})
	io.WriteString(w, string(b))
}

func get(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})
	key := r.FormValue("key")
	log.Println("GET", "v3", key)
	err := onkey(key, data)
	if err != nil {
		data["errorCode"] = 500
		data["message"] = err.Error()
	}
	var dataByte []byte
	dataByte, _ = json.Marshal(data)
	io.WriteString(w, string(dataByte))
}

func onkey(key string, data map[string]interface{}) (err error) {
	resp, err := client.Get(context.Background(), key)
	kv := resp.Kvs[0]
	sch := runtime.NewScheme()
	clientgoscheme.AddToScheme(sch)
	apiextv1beta1.AddToScheme(sch)
	decoder := serializer.NewCodecFactory(sch).UniversalDeserializer()
	encoder := jsonserializer.NewSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, true)
	obj, _, err := decoder.Decode(kv.Value, nil, nil)
	var value string
	if err != nil {
		value = string(kv.Value)
		err = nil
	} else {
		var buff bytes.Buffer
		err = encoder.Encode(obj, &buff)
		if err != nil {
			panic(err)
		}
		value = buff.String()
	}

	node := make(map[string]interface{})
	node["key"] = string(kv.Key)
	node["value"] = value
	node["dir"] = false
	node["ttl"] = getTTL(client, kv.Lease)
	node["createdIndex"] = kv.CreateRevision
	node["modifiedIndex"] = kv.ModRevision
	data["node"] = node
	return err

}

func getPath(w http.ResponseWriter, r *http.Request) {
	originKey := r.FormValue("key")

	log.Println("GET", "v3", originKey)
	var (
		data = make(map[string]interface{})
		all  = make(map[int][]map[string]interface{})
		min  int
		max  int
	)
	var (
		presp *clientv3.GetResponse
		err   error
	)

	if originKey != separator {
		presp, err = client.Get(context.Background(), originKey, clientv3.WithKeysOnly())
		if err != nil {
			data["errorCode"] = 500
			data["message"] = err.Error()
			dataByte, _ := json.Marshal(data)
			io.WriteString(w, string(dataByte))
			return
		}
	}
	if originKey == separator {
		min = 1
	} else {
		min = len(strings.Split(originKey, separator))
	}
	max = min
	all[min] = []map[string]interface{}{{"key": originKey}}
	if presp != nil && presp.Count != 0 {
		all[min][0]["value"] = string(presp.Kvs[0].Value)
		all[min][0]["ttl"] = getTTL(client, presp.Kvs[0].Lease)
		all[min][0]["createdIndex"] = presp.Kvs[0].CreateRevision
		all[min][0]["modifiedIndex"] = presp.Kvs[0].ModRevision
	}
	all[min][0]["nodes"] = make([]map[string]interface{}, 0)

	key := originKey
	var resp *clientv3.GetResponse
	resp, err = client.Get(context.Background(), key, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend), clientv3.WithKeysOnly())
	if err != nil {
		data["errorCode"] = 500
		data["message"] = err.Error()
		dataByte, _ := json.Marshal(data)
		io.WriteString(w, string(dataByte))
		return
	}

	for _, kv := range resp.Kvs {
		if string(kv.Key) == separator {
			continue
		}
		keys := strings.Split(string(kv.Key), separator) // /foo/bar
		for i := range keys {                            // ["", "foo", "bar"]
			k := strings.Join(keys[0:i+1], separator)
			if k == "" {
				continue
			}
			node := map[string]interface{}{"key": k}
			if node["key"].(string) == string(kv.Key) {
				node["value"] = string(kv.Value)
				if key == string(kv.Key) {
					node["ttl"] = getTTL(client, kv.Lease)
				} else {
					node["ttl"] = 0
				}
				node["createdIndex"] = kv.CreateRevision
				node["modifiedIndex"] = kv.ModRevision
			}
			level := len(strings.Split(k, separator))
			if _, ok := all[level]; !ok {
				all[level] = make([]map[string]interface{}, 0)
			}
			levelNodes := all[level]
			var isExist bool
			for _, n := range levelNodes {
				if n["key"].(string) == k {
					isExist = true
				}
			}
			if !isExist {
				node["nodes"] = make([]map[string]interface{}, 0)
				all[level] = append(all[level], node)
			}
		}
	}

	// parent-child mapping
	minus := len(strings.Split(key, separator))
	fmt.Printf("此时的max: %d min: %d key: %s minus: %d\n", max, min, key, minus)

	max = minus + 2

	for i := max; i > min; i-- {
		for _, a := range all[i] {
			for _, pa := range all[i-1] {
				if i == 2 {
					pa["nodes"] = append(pa["nodes"].([]map[string]interface{}), a)
					pa["dir"] = true
				} else {
					if strings.HasPrefix(a["key"].(string), pa["key"].(string)+separator) {
						pa["nodes"] = append(pa["nodes"].([]map[string]interface{}), a)
						pa["dir"] = true
					}
				}
			}
		}
	}
	data = all[min][0]
	dataByte, _ := json.Marshal(map[string]interface{}{"node": data})
	io.WriteString(w, string(dataByte))
}

func getTTL(cli *clientv3.Client, lease int64) int64 {
	resp, err := cli.Lease.TimeToLive(context.Background(), clientv3.LeaseID(lease))
	if err != nil {
		return 0
	}
	if resp.TTL == -1 {
		return 0
	}
	return resp.TTL
}

func newClient(host string) {
	var err error
	var tlsConfig *tls.Config
	if *cacert != "" {
		tlsInfo := transport.TLSInfo{
			CertFile:      *cert,
			KeyFile:       *keyfile,
			TrustedCAFile: *cacert,
		}
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			log.Println(err.Error())
		}
	}

	conf := clientv3.Config{
		Endpoints:          []string{host},
		DialTimeout:        time.Second * time.Duration(5),
		TLS:                tlsConfig,
		DialOptions:        []grpc.DialOption{grpc.WithBlock()},
		MaxCallSendMsgSize: 2 * 1024 * 1024,
	}
	client, err = clientv3.New(conf)
	if err != nil {
		panic(err)
	}
}

func getInfo() map[string]string {
	info := make(map[string]string)
	status, err := client.Status(context.Background(), host)
	if err != nil {
		log.Fatal(err)
	}
	mems, err := client.MemberList(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range mems.Members {
		if m.ID == status.Leader {
			info["version"] = status.Version
			info["size"] = humanize.Bytes(uint64(status.DbSize))
			info["name"] = m.GetName()
			break
		}
	}
	return info
}
