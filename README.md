Logo generated with:

- https://patorjk.com/software/taag/#p=display&f=House+Red&t=termfolio&x=none&v=4&h=4&w=80&we=false&ft=thedraw

Run with docker

```bash
docker run -d --name calculon -v "$PWD":/data -p 23234:23234 calculon
```

Access:

```
ssh -p 23234 localhost
```


Push local db:

```bash
fly ssh sftp put calculon.db /data/calculon.db.restore -a calculon

```

Rewire:
```bash
fly ssh console -a calculon -C "sh -c 'cd /data && rm -f calculon.db-wal calculon.db-shm && mv calculon.db.restore calculon.db'"

fly apps restart calculon
 ```
