# dkv

Library for insantiating and working with distributed KV storage nodes.

## Main classes diagram

```puml
struct Client{
    # Controller
    + Start(ctx) error
    + Get(key) (KVPair, error)
    + AddOrUpdate(key, val) error
    + Remove(key) error
    + Metrics() []metrics
}

struct Processor{
    + Start(ctx) error
    + Get(key) (KVPair, error)
    + AddOrUpdate(key, val) error
    + Remove(key) error
    + Metrics() []metrics
    # Merger
    # LocalKVRepo
    # RemoteKVRepo
}

interface Merger{
    + Merge(KVPair, KVPair) KVPair
}
struct LastTsMerger
LastTsMerger .up.|> Merger: implements

interface Controller{
    + Start(ctx, Processor) error
    + Metrics() []metrics
}
struct HttpController{
    # Processsor
}
HttpController .up.|> Controller: implements

interface LocalKVRepo{
    + Get(key) (VPair, error)
 + GetNoValue(key) (KVPair, error)
 + AddOrUpdate(KVPair, Merger) error
 + Remove(key) error
 + CheckTombstone(key) (ts, err)
 + GetAllNoValue() ([]KVPair, error)
    + Metrics() []metrics
}
struct BadgerLocalKVRepo
BadgerLocalKVRepo .up.|> LocalKVRepo: implements

interface RemoteKVRepo{
    + Get(ctx, hostname, key) (KVPair, error)
 + GetNoValue(ctx, hostname, key) (KVPair, error)
 + AddOrUpdate(ctx, hostname, KVPair) error
 + Remove(ctx, hostname, key) error
    + Metrics() []metrics
}
struct HttpRemoteKVRepo
HttpRemoteKVRepo .up.|> RemoteKVRepo: implements

Client --|> Processor: extends
Client --o Controller: uses
Controller --o Processor: uses
Processor --o Merger: uses
Processor --o LocalKVRepo: uses
Processor --o RemoteKVRepo: uses
```
