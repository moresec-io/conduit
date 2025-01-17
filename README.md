<p align=center>
<img src="./docs/diagrams/logo.png" width="30%" height="30%">
</p>

<div align="center">

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

</div>

Conduit是一个透明代理Mesh，为你的ToB集群间安全保驾护航。

## 特性

- **集群安全** 支持TLS和mTLS
- **性能无损** 使用Netfilter提供的透明代理，几乎无性能损耗
- **全场景代理** 支持简单Client/Server代理，也支持透明代理Mesh


## 使用

### Client-Server

![](./docs/diagrams/client-server.jpg)


配置做为客户端：

```yaml
client:
  enable: true
  network: tcp
  listen: 127.0.0.1:5052
  check_time: 60
  forward_table:
    - dst: :80
      dst_as: 127.0.0.1:80
      peer_index: 1
  peers:
    - index: 1
      network: tcp
      addresses:
        - 172.168.0.11:5053

log:
  maxsize: 10
  level: debug
  file: /opt/conduit/log/conduit.log
```

配置做为服务端

```yaml
server:
  enable: true
  listen:
    network: tcp
    addr: 172.168.0.11:5053

log:
  maxsize: 10
  level: debug
  file: /opt/conduit/log/conduit.log
```

此时所有在客户端访问:80端口都会经过172.168.0.11:5053访问到172.168.0.11上的127.0.0.1:80端口

### Mesh

![](./docs/diagrams/conduit.jpg)


Manager配置：

```yaml
conduit_manager:
  listen:
   network: "tcp"
   addr: "0.0.0.0:5051"

db:
  driver: sqlite
  address: /opt/conduit/data/
  db: manager.db
  debug: false

cert: # cert strategy for conduits
  ca:
    not_after: 1,0,0 # 1 year 0 month 0 day
    common_name: "conduit.com"
  cert:
    not_after: 1,0,0
    common_name: "conduit.com"
    organization: "moresec.com"

log:
  maxsize: 10
  level: info
  file: /opt/conduit/log/manager.log
```


配置做为客户端和服务端：

```yaml
manager:
  enable: true
  dial:
    network: tcp
    addresses:
      - 172.168.0.17:5051

server:
  enable: true
  listen:
    network: tcp
    addr: 172.168.0.11:5053

client:
  enable: true
  network: tcp
  listen: 127.0.0.1:5052
  check_time: 60

log:
  maxsize: 10
  level: debug
  file: /opt/conduit/log/conduit.log
```

如果配置了Manager，那么client和server之间默认使用mTLS安全通道

Released under the [Apache License 2.0](https://github.com/moresec-io/conduit/blob/main/LICENSE)