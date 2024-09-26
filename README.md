# whip - simple and fast devops automation

![order, order!](doc/order-order.webp)

Whip your servers into line. A _fast_ and _simple_ Ansible replacement optimized for projects with 1 to 20 servers.

# Install

```
base=https://github.com/gwillem/whip/releases/latest/download/whip
curl -L $base-$(uname -s)-$(uname -m).gz|gzip -d>whip&&chmod +x whip
./whip version
```

# Demo

> [!NOTE]
> Keep this in mind.

# Features

| Finished   | Planned            | NOT planned\* |
| ---------- | ------------------ | ------------- |
| ssh auth   | external inventory | non-linux     |
| ssh agent  | facts              | sudo / become |
| apt        | pip / env          | ssh passwords |
| file/copy  | roles / includes   | local_action  |
| shell      | rpm, yum, pacman   | with_xxx      |
| command    | get_url            | delegate_to   |
| lineinfile | apt_repository     | set_fact      |
| vars       | user               | assert        |
| templates  | mysql              | stat          |
| vault      | postgresql         | debug         |

# Philosophy

How will Whip _stay_ fast and simple?

Only build features that satisfy 95% of use cases. Convention over configuration. Support top used modules only. Only support Linux servers and Linux/Mac clients.

Eliminate unnecessary SSH round trips: Ansibles biggest delay is caused by tasks that are sent one by one. Whip bundles tasks into a single job.

# But why?

Ansible started out as fast and simple too. Compared to the popular configuration management systems at the time (Puppet, Chef, CFEngine), it was a breeze of fresh air. Simple configuration files, easy to learn, effective documentation, simple push architecture.

Until version 2 or so. After the RedHat acquisition, Ansible grew into commercial bloatware. RedHat got rid of the old objectives page (Simple, Fast) and replaced it with corporate marketing fluff. The task parameter documentation is hidden behind white paper downloads. Core modules have grown to support 20 extra options to support esoteric use cases. And above all, its once legendary speed is gone. Ansible feels sluggish today.

Ansible has grown too complex, as illustrated by this Hacker News comment:

> Any sufficiently complicated configuration language contains an ad hoc, informally-specified, bug-ridden, slow implementation of a Turing complete programming language. (jasim @ HN)

# Other reading

- [Top Ansible tasks](https://mike42.me/blog/2019-01-the-top-100-ansible-modules)
- [I'm done with Red Hat](https://www.jeffgeerling.com/blog/2023/im-done-red-hat-enterprise-linux)
- [Is Ansible turing complete?](https://stackoverflow.com/questions/40127586/is-ansible-turing-complete)
- [Ansible's YAML file is essentially code](https://news.ycombinator.com/item?id=16238005)
- [Configuration complexity clock](http://mikehadlow.blogspot.com/2012/05/configuration-complexity-clock.html?m=1)
- [Original Ansible site: simple and efficient](https://web.archive.org/web/20130314042108/http://www.ansibleworks.com/)
- Some recent config mgt alternatives:
  - [gossh: declarative config management using Go](https://github.com/krilor/gossh)
  - [JetPorch](https://github.com/jetporch/jetporch_docs/blob/main/SUMMARY.md) ([launched](https://laserllama.substack.com/p/a-new-it-automation-project-moving) and [discontinued](https://web.archive.org/web/20231230013721/https://jetporch.substack.com/p/discontinuing-jet))
  - [Ploy](https://github.com/davesavic/ploy) Jan 2024, not Ansible compatible
  - [Bruce](https://github.com/brucedom/bruce) since Apr 2023, not Ansible compatible
  - [mgmt](https://github.com/purpleidea/mgmt/) since 2016, full featured, high complexity, not Ansible compatible
  - [Tiron](https://github.com/lapce/tiron) some Ansible runners, written in Rust, uses HCL instead of Yaml
  - [Sparky](https://github.com/melezhik/sparky) [see also](https://dev.to/melezhik/sparky-simple-and-efficient-alternative-to-ansible-1fod)

# FAQ

#### Is Whip designed to be an Ansible replacement (backwards compatible) or to be a better solution to the same problem?

The latter, however we stick to most of Ansible's verbiage to ease a transition.

#### Isn't everybody using Docker, Kubernetes and Kamal etc these days?

[Not really](https://trends.google.com/trends/explore?date=all&q=ansible).

#### Why is there an embedded build?

To support different architectures between host and client

# Changes from Ansible syntax

- tree module with state per line
- apt module with state per line
- "unless" for command and shell
