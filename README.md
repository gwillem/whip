# Chief Whip - simple and fast devops automation

![order, order!](doc/order-order.webp)

Whip your servers into line. Chief Whip is a _fast_ and _simple_ Ansible replacement optimized for projects with 1 to 20 servers.

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

# Demo

> [!NOTE]
> Keep this in mind.

# Philosophy

How will Chief Whip _stay_ fast and simple? By will only contain features that satisfy 95% of use cases. Convention over configuration.

Fast!

1. Eliminate unnecessary SSH round trips: Ansibles biggest delay is caused by tasks that are sent one by one. Chief Whip bundles tasks into a single job.
2. Implemented in Golang instead of Python

Simple!

# But why?

I really loved Ansible. Compared to the popular configuration management systems at the time (Puppet, Chef, CFEngine), it was a breeze of fresh air. Simple configuration files, easy to learn, effective documentation, simple push architecture. My team used it to manage some 2k+ servers without a fuss.

Until version 2 or so. After the RedHat acquisition, Ansible has quickly grown into commercial bloatware. It's funny how RedHat got rid of the old objectives page (Simple, Fast) and replaced it with a corporate bog of marketing fluff. The task parameter documentation is hidden behind compulsory white paper downloads. Core modules have grown to support 20 extra options to support esoteric use cases. And above all, its once legendary speed is gone. Ansible feels sluggish today.

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

# FAQ

#### Is Chief Whip designed to be an Ansible replacement (backwards compatible) or to be a better solution to the same problem?

The latter, however we stick to most of Ansible's verbiage to ease a transition.

#### Isn't everybody using Docker, Kubernetes etc these days?

[Nope](https://trends.google.com/trends/explore?date=all&q=ansible).

#### Why is there an embedded build?

To support different architectures between host and client

# Changes from Ansible syntax

- tree module with state per line
- apt module with state per line
- "unless" for command and shell
