k8s的etcd数据查看工具
配置config.yaml后 go run main.go运行
```bash
curl http://0.0.0.0:8080/ui/
```
已知问题:为了简化数据量,只返回了三级的数据,直接点击三角形展开会有问题,建议直接点击文件夹
![示例.jpg](./示例.jpg)