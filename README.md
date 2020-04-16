### Skip List in IPLD

This repo implements skip list data-structure using IPLD.
IPLD skip list is append only data-structure allowing for `O(log(n))` lookups
and `O(1)` append.

This is achieved with minimal size increase (1 additional link on average).
On the small scale skip list looks like this:
![](./docs/skip-list.jpg)


#### Licence

MIT + Apache 2.0
