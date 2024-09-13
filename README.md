Registry Cache is an opinionaned cache for OCI compliant images with support for multi registries.

The aim is to replace Dragonfly with a more reliable solution.

To understand how it works, see the below request flow:

[[request flow diagram]]

## Architecture:


**PROXY ARCHITECTURE:**

[[software design diagram]]

**DESCRIPTION:**

The 4 main components are: cache index, garbage collector, proxy and workers.
</br>
</br>
**The cache index** keeps in memory references between "requests", "users" and files in the filesystem. 
It serves as a "database" to understand which file a user request should receive.

The structure of the index is as follow:

$CacheKey -> $PathToData

The $CacheKey is the SHA256 of the request. This is part of the URL path when requesting images manifests or layers 
(e.g: `https://myregistry.com/v2/<$image>/blobs/sha256:<layer-sha256>`)

Permissions are always checked using a head request when requesting layers (/blobs/sha256:...) or manifests (/manifests/sha256:...).

**The garbage collector** is a go routine that runs every X minutes and performs the following:

- check and ensures that disk usage is below the disk.maxSize configured.
- removes corrupted files (for layers only).
- removes empty cache keys from index (cache keys without the underlying files)
- removes undesired files (!= layers/manifests)
- removes orphan files (files without metadata associated)
- removes files that reached max-unused or max-age.

**The proxy** is a Go webserver that sends requests to the workers and streams responses to the clients

**Workers** are go routines waiting for work to do. They fetch data from the upstream and optimize and reduce the number of necessary requests to it.
The workers are the only component connecting to the upstream registry.
When multiple requests for the same resources are submitted ONLY one worker talks to the upstream registry, the rest of the workers either wait or pick up new (different) work to do.

**Example Config**:

```
dataPath: /cache/
server:
  workers: 10
  streamers: 100
  upstreamTimeout: 1m
  timeout: 1m
  address: 0.0.0.0:7000
  defaultBackend:
    host: myregistry.com
    scheme: https

  upstreamRules: 
  - regex: "^(.+).dragonfly-local.c3.zone:7000$"
    host: "$group1.myregistry.com"
    scheme: "https"

  tls:
    certPath: ./config/localhost.crt
    keyPath: ./config/localhost.key
    caPath: ./config/ca.crt

metrics:
  address: 0.0.0.0:3000

gc:
  disk:
    maxSize: 1TB
  interval: 20m
  layers:
    checkSHA: true
    maxAge: 1h
    maxUnused: 30m
  manifests:
    maxAge: 10m
    maxUnused: 5m
```

## FAQ

- Why not using a simple NGINX proxy to cache?

NGINX cache is not optimized for image layers caching and to ensure the right level of security, we would have to create new data for every new user. Which on the farm is not feasible.

- Oh no, is the data and index local to the pod? Are data duplicated on every pod?

Yes. This app is designed to run on a few nodes and store cached data locally. On the Armada farm it should run as deamonset on a small subset of nodes (3-5).
However, see section below for "Future improvements" ;).

- What happens if my pod gets restarted?

The app will recreate the in-memory index based on the files stored on the node. 
So it is reccomended to use HostPath in the pod spec or a PVC to store the cached data.

##  Future improvements

We should optimize registry-cache by transitioning the in-memory index from being replicated across each instance to a centralized Redis cluster. 
This consolidation will improve efficiency and combined with a multi-tier caching system will allow shared data access from any replica.

**Shared Redis Cluster**: 

Centralizing the in-memory index within a Redis cluster ensures synchronized access across all replicas, reducing redundancy and data replication.

**Multiple Cache Levels**:

L0 - In-Memory Cache: Frequently accessed files or data layers are cached in memory, optimizing read operations. Linux paging mechanisms can be leveraged to efficiently manage this cache layer.

L1 - Local Disk Cache: Cached files are stored on the local disk of each instance, enabling rapid access and reducing latency.

L2 - VAST or Distributed Storage: Files not present in the local replica's cache (but present in the shared index) are sourced from VAST or distributed storage. This approach ensures accessibility to files not readily available locally, with VAST serving as a high-performance alternative to Artifactory.
