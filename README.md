# govern

declarative idempotent golang config management and orchestration

Base all the functionality naming off of "govern"ment terminology. i.e.

* LAWS - description of systems (users to create, pkgs to install, etc)
* FACTS - facts about systems (# of cpus, memory, distro/version, etc) = facts

I know, cute isn't it


## Currently Supported Laws

i.e. what can I control on my system

* Users - system users
* Groups - system groups
* Packages - install/~remove~ packages
* Containers - run docker containers
* Scripts - run scripts on a system
* Files - write file contents
* Mounts - add filesystem mounts (including network filesystems, etc)
* Services - start services and add services to runlevels

## Currently Supported Facts

* Hostname
* UID
* EUID
* GID
* EGID
* Groups
* PID
* PPID
* Environ
* SystemUUID
* MemoryTotal
* InitSystem
* CPUInfo
* Distro
* Network

## TODO

* go embed config files

### General
* dependency resolver
* parallel apply
  * where that makes sense (i.e. not during package install)

### containers
* support more runtimes (containerd, that redhat one that nobody uses, etc)
* watchtower like functionality (watch for updated image tag)

### files
* add go templating?
