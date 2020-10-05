# govern

golang config management and orchestration

Base all the functionality naming off of "govern"ment terminology. i.e.

  * LAWS - description of systems (users to create, pkgs to install, etc)
  * FACTS - facts about systems (# of cpus, memory, distro/version, etc) = facts

I know, cute isn't it

## TODO

### General
* dependency resolver
* parallel apply
  * where that makes sense (i.e. not during package install)

### containers
* watchtower like functionality (watch for updated image tag)