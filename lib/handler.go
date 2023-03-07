package lib

import (
	"context"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/phuslu/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

var client *clientv3.Client
var config Config
var sch *runtime.Scheme

func Connect(c *gin.Context) {
	info := make(map[string]string)
	host := strings.Split(config.Host, ",")[0]
	status, err := client.Status(context.Background(), host)
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
	log.Info().Msgf("请求的key为: %s", key)
	resp, err := client.Get(context.Background(), key)
	if err != nil {
		ResultErr(c, err)
		return
	}
	kv := resp.Kvs[0]
	node := make(map[string]any)
	node["key"] = string(kv.Key)
	node["value"] = decode(kv.Value)
	node["dir"] = false
	node["ttl"] = getTTL(client, kv.Lease)
	node["createdIndex"] = kv.CreateRevision
	node["modifiedIndex"] = kv.ModRevision
	data["node"] = node
	if err != nil {
		ResultErr(c, err)
		return
	}
	c.JSON(200, data)
}

func GetPath(c *gin.Context) {
	key := c.Query("key")

	log.Info().Msgf("请求的路径为: %s", key)
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

	if key != "/" {
		presp, err = client.Get(context.Background(), key, clientv3.WithKeysOnly())
		if err != nil {
			ResultErr(c, err)
		}
	}
	if key == "/" {
		min = 1
	} else {
		min = len(strings.Split(key, "/"))
	}
	max = min
	all[min] = []map[string]interface{}{{"key": key}}
	if presp != nil && presp.Count != 0 {
		all[min][0]["value"] = string(presp.Kvs[0].Value)
		all[min][0]["ttl"] = getTTL(client, presp.Kvs[0].Lease)
		all[min][0]["createdIndex"] = presp.Kvs[0].CreateRevision
		all[min][0]["modifiedIndex"] = presp.Kvs[0].ModRevision
	}
	all[min][0]["nodes"] = make([]map[string]interface{}, 0)

	resp, err := client.Get(context.Background(), key, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend), clientv3.WithKeysOnly())
	if err != nil {
		ResultErr(c, err)
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
