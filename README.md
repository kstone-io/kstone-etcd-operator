# kstone-etcd-operator

---
kstone-etcd-operator is a subproject of etcd cluster management platform [kstone](https://github.com/tkestack/kstone).
It's inspired by [etcd-operator](https://github.com/coreos/etcd-operator). And has more complete support for persistent
storage and better disaster tolerance.

## Features

* etcd cluster create and destroy
* Persistent storage support
* High availability and disaster tolerance
* etcd cluster upgrade
* Learner support
* Horizontal Scaling & Vertical Scaling
* HTTPS Support
* Custom parameters

### TODO List

- [ ] Non-persistent storage support
- [ ] Backup and Restore support
- [ ] Cluster Migrate
- [ ] Add e2e test

## Developing

### Build

``` shell
mkdir -p ~/tkestack
cd ~/tkestack
git clone https://github.com/tkestack/kstone-etcd-operator
cd kstone-etcd-operator
make
```

## Contact

For any question or support, feel free to contact us via:
- Join [#Kstone Slack channel](https://join.slack.com/t/w1639233173-qqx590963/shared_invite/zt-109muo6i9-0kTUQphSVFlwOSW7CgtrGw)
- Join WeChat Group Discussion (Join the group by adding kstone assistant WeChat and reply "kstone")

<div align="center">
  <img src="docs/images/kstone_assistant.jpg" width=20% title="Kstone_assistant WeChat">
</div>

## Community

* You are encouraged to communicate most things via
  GitHub [issues](https://github.com/tkestack/kstone-etcd-operator/issues/new/choose)
  or [pull requests](https://github.com/tkestack/kstone-etcd-operator/pulls).

## Licensing

kstone-etcd-operator is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license
text.