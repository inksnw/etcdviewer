package lib

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"strings"
	"time"
)

func ResultErr(c *gin.Context, err error) {
	data := make(map[string]any)
	data["errorCode"] = 500
	data["message"] = err.Error()
	c.JSON(500, data)
}

func decode(v []byte) (value string) {
	decoder := serializer.NewCodecFactory(sch).UniversalDeserializer()
	opts := jsonserializer.SerializerOptions{Pretty: true}
	encoder := jsonserializer.NewSerializerWithOptions(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, opts)
	obj, _, err := decoder.Decode(v, nil, nil)
	if err != nil {
		value = string(v)
		return value
	}
	var buff bytes.Buffer
	err = encoder.Encode(obj, &buff)
	if err != nil {
		value = string(v)
		return value
	}
	return buff.String()
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
func InitSch() {
	sch = runtime.NewScheme()
	scheme.AddToScheme(sch)
	apiextv1beta1.AddToScheme(sch)
}

func InitClient() {
	tlsInfo := transport.TLSInfo{
		CertFile:      config.Cert,
		KeyFile:       config.Key,
		TrustedCAFile: config.CA,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	Check(err)
	hosts := strings.Split(config.Host, ",")
	conf := clientv3.Config{
		Endpoints:          hosts,
		DialTimeout:        time.Second * 5,
		TLS:                tlsConfig,
		DialOptions:        []grpc.DialOption{grpc.WithBlock()},
		MaxCallSendMsgSize: 2 * 1024 * 1024,
	}
	client, err = clientv3.New(conf)
	Check(err)
}
