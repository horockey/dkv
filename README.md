# dkv

Библиотека для развертывания и взаимодействия с узлами распределенного KV-хранилища.

## Основные классы

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
    # Merger
    # LocalKVRepo
    # RemoteKVRepo
    # Hashring
    # Discovery

    + Start(ctx) error
    + Get(key) (KVPair, error)
    + AddOrUpdate(key, val) error
    + Remove(key) error
    + Metrics() []metrics
}

interface Discovery{
    + Register(ctx, hostname, updCallBack, meta) error
    + Deregister(ctx) error
    + GetNodes(ctx) ([]Node, error)
}

struct Hashring{
    + HashFunc
}

Interface HashFunc{
    + func([]byte) HashKey
}

Hashring ..o HashFunc: uses

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
Client -left-o Controller: uses
Controller --o Processor: uses
Processor --o Merger: uses
Processor --o LocalKVRepo: uses
Processor --o RemoteKVRepo: uses
Processor --o Hashring: uses
Processor --o Discovery: uses
```

* **LocalKVRepo** - интерфейс локального хранилища. К нему будут проходить обращения, когда текущий узел будет для ключа держателем или репликой.

  * Библиотека предоставляет стандартную реализацию этого интерфейса - персистентное хранилище на основе файловой системы. Под капотом используется [badger](https://github.com/hypermodeinc/badger).

* **RemoteKVRepo** - интерфейс, обобщающий обращения к другим узлам системы. Будет использоваться, когда текущий узел не будет являться держателем ключа.

  * Библиотека предоставляет стандартную реализацию этого интерфейса - обертку над HTTP клиентом для обращения к соответствующим контроллерам на других узлах.

* **Controller** - интерфейс, обобщающий сущность, ответственную запрослушивание обращений извне к узлу.

  * Реализация по умолчанию - HTTP контроллер.

* **Merger** - Интерфейс, обобщающий механизм слияния старой и новой версии ключ-значения.

  * Реализация по умолчанию - новая версия затирает старую.

* **Discovery** - интерфейс клиента сервиса типа [Discovery](https://habr.com/ru/articles/487706/). Подразумеваетналичие такого сервиса и его постоянную доступность. Реализации по умолчанию быть не может, но как вариант решения предлагается использовать клиента [horockey/service_discovery](https://github.com/horockey/service_discovery).

* **Hashring** - сущность, описывающая хэш-кольцо - основную сущность алгоритма консистентного хэширования (почитать можно [ТУТ](https://www.toptal.com/big-data/consistent-hashing) и [ТУТ](https://habr.com/ru/companies/mygames/articles/669390/)). Для непоредствено хэширования используется HashFunc - функция оговоренной сигнатуры. По умолчанию это функция-оберкта над MD5 хэшированием.

* **Processor** - центральная сущность библиотеки, содержащая в себе бизнес-логику ее работы. Принимает обращения от контроллера и клиента, обращается к нижестоящим адаптерам, обеспечивает запись реплик, транзакционность операций, при необходимости осуществляет их откаты.

* **Client** - Сущность - обертка над процессором. Обеспечивает старт процессора и контроллера, предоставляет возможность разработчику взаимодейтвовать с системой.

## Пример использования

```go
package main

import (
 "context"
 "errors"
 "fmt"
 "math/rand/v2"
 "net/http"
 "os"
 "os/signal"
 "strconv"
 "sync"
 "syscall"
 "time"

 "github.com/google/uuid"
 "github.com/horockey/dkv"
 "github.com/horockey/dkv_monkey_service/internal/model"
 serdisc "github.com/horockey/service_discovery/api"
 "github.com/rs/zerolog"
)

const TotalStorageCap = 1_000_000

func main() {
 serv := http.Server{
  Addr: "0.0.0.0:80",
 }

 logger := zerolog.New(zerolog.ConsoleWriter{
  Out:        os.Stdout,
  TimeFormat: time.RFC3339,
 }).With().Timestamp().Logger()

 disc, err := serdisc.NewClient(
  "dkv_monkey_service",
  "http://discovery:6500",
  "ak",
  &serv,
  logger.
   With().
   Str("scope", "serdisc_client").
   Logger(),
 )
 if err != nil {
  logger.
   Fatal().
   Err(fmt.Errorf("creating serdisc client: %w", err)).
   Send()
 }

 hostname, _ := os.Hostname()
 cl, err := dkv.NewClient(
  "dkv_ak",
  hostname,
  &model.DiscoveryImpl{Cl: disc},
  dkv.WithServicePort[model.Value](7000),
  dkv.WithLogger[model.Value](
   logger.
    With().
    Str("scope", "dkv_client").
    Logger(),
  ),
 )
 if err != nil {
  logger.
   Fatal().
   Err(fmt.Errorf("creating dkv client: %w", err)).
   Send()
 }

 ctx, cancel := signal.NotifyContext(
  context.Background(),
  syscall.SIGTERM,
  syscall.SIGABRT,
  syscall.SIGINT,
  syscall.SIGQUIT,
  syscall.SIGKILL,
 )
 defer cancel()

 var wg sync.WaitGroup

 wg.Add(1)
 go func() {
  defer wg.Done()
  if err := serv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
   logger.
    Error().
    Err(fmt.Errorf("running http_server")).
    Send()
  }
 }()

 wg.Add(1)
 go func() {
  defer wg.Done()

  if err := cl.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
   logger.
    Error().
    Err(fmt.Errorf("running dkv client: %w", err)).
    Send()
   cancel()
  }
 }()

 wg.Add(1)
 go func() {
  defer wg.Done()

  time.Sleep(time.Second * 5)

  for {
   select {
   case <-ctx.Done():
    return
   default:
    idx := rand.IntN(TotalStorageCap)
    key := "monkey_" + strconv.Itoa(idx)
    value := model.Value{
     Foo: uuid.NewString(),
     Bar: uuid.NewString(),
    }
    action := rand.IntN(2)
    switch action {
    case 0:
     // write
     if err := cl.AddOrUpdate(ctx, key, value); err != nil {
      logger.Error().Err(fmt.Errorf("writing to client: %w", err)).Send()
     }
    case 1:
     // read
     if _, err := cl.Get(ctx, key); err != nil && !errors.Is(err, dkv.ErrKeyNotFoundError{Key: key}) {
      logger.Error().Err(fmt.Errorf("reading from client: %w", err)).Send()
     }
    }

    time.Sleep(time.Millisecond * 100)
   }
  }
 }()

 logger.Info().Msg("Service started")

 <-ctx.Done()
 _ = serv.Close()
 wg.Wait()

 logger.Info().Msg("Service stopped")
}

```

## Диаграмма последовательности

```plantuml
Participant "Client\nOR\nHTTP controller" as CL
Entity Processor
Entity Hashring
Database LocalKVRepo
Entity RemoteKVRepo
Entity Merger

CL --> Processor: CRUD операция
Processor <--> Hashring: Получить держателя\nи реплики

alt Мы не держатель и не реплика
    Processor --> RemoteKVRepo: Передать держателю
end
alt Мы держатель
    Processor --> LocalKVRepo: Провести операцию
    Processor --> RemoteKVRepo: Провести на репликах
end
alt Мы реплика
    Processor --> LocalKVRepo: Провести операцию
end

alt Операция записи
    LocalKVRepo <--> Merger: Произвести слияние\nстарой и новой версии
end

CL <-- Processor: Результат выполнения
```
