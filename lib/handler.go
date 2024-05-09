package lib

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/phuslu/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

var client *clientv3.Client
var config Config
var sch *runtime.Scheme

func Get(c *gin.Context) {
	key := c.Query("key")
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
	result := gin.H{
		"node": map[string]any{
			"key":           string(kv.Key),
			"value":         decode(kv.Value),
			"dir":           false,
			"createdIndex":  kv.CreateRevision,
			"modifiedIndex": kv.ModRevision,
		},
	}
	c.JSON(200, result)
}
func GetPath(c *gin.Context) {
	key := c.Query("key")
	log.Info().Msgf("请求的路径为: %s", key)

	data, all, min, maxLevel := prepareData(key)

	resp, err := client.Get(context.Background(), key, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend), clientv3.WithKeysOnly())
	if err != nil {
		ResultErr(c, err)
		return
	}

	maxLevel = processKeys(resp, all, key, maxLevel)

	for i := maxLevel; i > min; i-- {
		processNodes(all, i)
	}

	data = all[min][0]
	c.JSON(200, gin.H{"node": data})
}

func prepareData(key string) (map[string]any, map[int][]map[string]any, int, int) {
	data := make(map[string]any)
	all := make(map[int][]map[string]any)
	min := 1
	if key != "/" {
		min = len(strings.Split(key, "/"))
	}
	all[min] = []map[string]any{{"key": key}}
	all[min][0]["nodes"] = make([]map[string]any, 0)
	return data, all, min, 0
}

func processKeys(resp *clientv3.GetResponse, all map[int][]map[string]any, key string, maxLevel int) int {
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
	return maxLevel
}

func processNodes(all map[int][]map[string]any, i int) {
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
