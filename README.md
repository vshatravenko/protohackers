# Protohackers Implementations

This repository implements [Protohackers](https://protohackers.com/) challenges.

## Challenges

To run or deploy a given challenge, set the `TARGET_BINARY` env var
to that challenge's lowercased and underscored name
(e.g. `smoke_test` for Smoke Test), then use the desired `make` task

```sh
export TARGET_BINARY=smoke_test
# Run the implementation locally
make run
# Or deploy it to fly.io
make deploy
```

You can run `make help` to display the most commonly used `make` targets.

The currently implemented challenges are:

1. [Smoke Test](https://protohackers.com/problem/0) - [smoke_test](./cmd/smoke_test)
1. [Prime Time](https://protohackers.com/problem/1) - [prime_time](./cmd/prime_time)
1. [Means to an End](https://protohackers.com/problem/2) - [means_end](./cmd/means_end)
1. [Budget Chat](https://protohackers.com/problem/3) - [budget_chat](./cmd/budget_chat)
1. [Unusual Database Program](https://protohackers.com/problem/4) - [unusual_database](./cmd/unusual_database)
1. [Mob in the Middle](https://protohackers.com/problem/5) - [mob_middle](./cmd/mob_middle)
