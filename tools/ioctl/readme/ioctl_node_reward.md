## ioctl node reward

Query rewards

### Synopsis

ioctl node reward pool returns unclaimed and available Rewards in fund pool.
TotalUnclaimed is the amount of all delegates that have been issued but are not claimed;
TotalAvailable is the amount of balance that has not been issued to anyone.

ioctl node reward unclaimed [ALIAS|DELEGATE_ADDRESS] returns unclaimed rewards of a specific delegate.

```
ioctl node reward unclaimed|pool [ALIAS|DELEGATE_ADDRESS] [flags]
```

### Options

```
  -h, --help   help for reward
```

### Options inherited from parent commands

```
      --endpoint string        set endpoint for once
      --insecure               insecure connection for once
  -o, --output-format string   output format
```

### SEE ALSO

* [ioctl node](ioctl_node.md)	 - Deal with nodes of IoTeX blockchain

###### Auto generated by docgen on 7-Mar-2022
