This is simple Go workspace to show how DAPR actors and pubsub services can be used in Go.

## Setup

```bash
go mod init github.com/khaledhikmat/campaign-manager-gist
```

## Uprade Go Version

```bash
go mod edit -go 1.22
go mod tidy
```

## Change Module Name

```bash
go mod edit -moduel <new_name>
```

But remember to refactor the import and do `go mod tidy`.

## Start and Stop

```bash
chmod 777 ./start.sh
chmod 777 ./stop.sh
./start.sh
./stop.sh
```

Please note the `stop` script has to be executed to avoid risking the DAPR srevice port be occupied. So u can have two terminals: one to start and one to stop.

## Actors

I noticed that the 1st actor invocation causes the following error...but it does seem to work anyway:

```
== APP == 2024/01/15 14:10:55 method GetStateManager is illegal, err = the latest return type actor.StateManagerContext of method "GetStateManager" is not error, just skip it
== APP == 2024/01/15 14:10:55 method ID is illegal, err = the latest return type string of method "ID" is not error, just skip it
== APP == 2024/01/15 14:10:55 method SetID is illegal, err = num out invalid, just skip it
== APP == 2024/01/15 14:10:55 method SetStateManager is illegal, err = num out invalid, just skip it
== APP == 2024/01/15 14:10:55 method Type is illegal, err = the latest return type string of method "Type" is not error, just skip it
```

Do not really like this....but this is how it is currently working.

## Redis cli

Access Docker CLI:

```bash
docker exec -it dapr_redis redis-cli
```

- Clean Redis: 
```bash
FLUSHALL
```

- Make sure:
```bash
KEYS *
```

- Get key value:
```bash
HGET campaign-manager-core||CampaignActorType||1000||main data
```

If running in k8s, do `docker ps` to discover the container name of the REDIS running in K8s. and then do the above docker command:

```bash
docker exec -it k8s_redis_redis-75db659ddc-q6jfn_dapr-storemanager_b116ad62-7b4e-4a75-968f-39f84ce8a16c_0 redis-cli
```

