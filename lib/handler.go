package lib

import (
	"context"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/phuslu/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
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
	if len(resp.Kvs) == 0 {
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
		data     = make(map[string]any)
		all      = make(map[int][]map[string]any)
		min      int
		maxLevel int
	)

	if key == "/" {
		min = 1
	}
	if key != "/" {
		min = len(strings.Split(key, "/"))
	}

	all[min] = []map[string]any{{"key": key}}
	all[min][0]["nodes"] = make([]map[string]any, 0)

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

		for i := range keys { // ["", "foo", "bar"]
			k := strings.Join(keys[0:i+1], "/")
			if k == "" {
				continue
			}
			node := map[string]any{"key": k}
			level := len(strings.Split(k, "/"))
			if level > maxLevel {
				maxLevel = level
			}
			if _, ok := all[level]; !ok {
				all[level] = make([]map[string]any, 0)
			}

			if level > len(strings.Split(key, "/"))+2 {
				break
			}

			var isExist bool
			for _, n := range all[level] {
				if n["key"].(string) == k {
					isExist = true
				}
			}
			if !isExist {
				node["nodes"] = make([]map[string]any, 0)
				all[level] = append(all[level], node)
			}
		}
	}
	marshal, err := json.Marshal(all)
	if err != nil {
		return
	}
	log.Info().Msgf("all: %s", marshal)

	for i := maxLevel; i > min; i-- {
		for _, child := range all[i] {
			for _, pre := range all[i-1] {
				childKey := child["key"].(string)
				preKey := pre["key"].(string) + "/"
				if i == 2 {
					pre["nodes"] = append(pre["nodes"].([]map[string]any), child)
					pre["dir"] = true
				} else {
					if strings.HasPrefix(childKey, preKey) {
						pre["nodes"] = append(pre["nodes"].([]map[string]any), child)
						pre["dir"] = true
					}
				}
			}
		}
	}
	data = all[min][0]
	c.JSON(200, gin.H{"node": data})
}
