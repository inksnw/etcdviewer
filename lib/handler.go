package lib

import (
	"bytes"
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/scheme"
	"log"
	"strings"
	"time"
)

var client *clientv3.Client
var config Config

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

func InitClient() {
	InitConfig()
	tlsInfo := transport.TLSInfo{
		CertFile:      config.Cert,
		KeyFile:       config.Key,
		TrustedCAFile: config.CA,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	Check(err)
	conf := clientv3.Config{
		Endpoints:          []string{config.Host},
		DialTimeout:        time.Second * 5,
		TLS:                tlsConfig,
		DialOptions:        []grpc.DialOption{grpc.WithBlock()},
		MaxCallSendMsgSize: 2 * 1024 * 1024,
	}
	client, err = clientv3.New(conf)
	if err != nil {
		panic(err)
	}
}

func Connect(c *gin.Context) {
	info := make(map[string]string)
	status, err := client.Status(context.Background(), config.Host)
	Check(err)
	memberList, err := client.MemberList(context.Background())
	Check(err)
	for _, m := range memberList.Members {
		if m.ID == status.Leader {
			info["version"] = status.Version
			info["size"] = humanize.Bytes(uint64(status.DbSize))
			info["name"] = m.GetName()
			break
		}
	}
	c.JSON(200, gin.H{
		"info":   info,
		"status": "running",
	})
}

func Get(c *gin.Context) {
	data := make(map[string]any)
	key := c.Query("key")
	log.Println("GET", "v3", key)
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

	node := make(map[string]any)
	node["key"] = string(kv.Key)
	node["value"] = value
	node["dir"] = false
	node["ttl"] = getTTL(client, kv.Lease)
	node["createdIndex"] = kv.CreateRevision
	node["modifiedIndex"] = kv.ModRevision
	data["node"] = node
	if err != nil {
		data["errorCode"] = 500
		data["message"] = err.Error()
	}
	c.JSON(200, data)
}

func GetPath(c *gin.Context) {
	originKey := c.Query("key")

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

	if originKey != "/" {
		presp, err = client.Get(context.Background(), originKey, clientv3.WithKeysOnly())
		if err != nil {
			data["errorCode"] = 500
			data["message"] = err.Error()
			c.JSON(500, data)
			return
		}
	}
	if originKey == "/" {
		min = 1
	} else {
		min = len(strings.Split(originKey, "/"))
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
		c.JSON(500, data)
		return
	}

	for _, kv := range resp.Kvs {
		if string(kv.Key) == "/" {
			continue
		}
		keys := strings.Split(string(kv.Key), "/") // /foo/bar
		for i := range keys {                      // ["", "foo", "bar"]
			k := strings.Join(keys[0:i+1], "/")
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
			level := len(strings.Split(k, "/"))
			if level > max {
				max = level
			}
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
	minus := len(strings.Split(key, "/"))
	fmt.Printf("此时的max: %d min: %d key: %s minus: %d\n", max, min, key, minus)

	//max = minus + 2

	for i := max; i > min; i-- {
		for _, a := range all[i] {
			for _, pa := range all[i-1] {
				if i == 2 {
					pa["nodes"] = append(pa["nodes"].([]map[string]interface{}), a)
					pa["dir"] = true
				} else {
					if strings.HasPrefix(a["key"].(string), pa["key"].(string)+"/") {
						pa["nodes"] = append(pa["nodes"].([]map[string]interface{}), a)
						pa["dir"] = true
					}
				}
			}
		}
	}
	data = all[min][0]
	rv := map[string]interface{}{"node": data}
	c.JSON(200, rv)
}
