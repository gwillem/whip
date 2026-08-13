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

![](https://buq.eu/screenshots/40234b57e57fda7399a2698a.png)

| Finished          | Planned            | NOT planned\* |
| ----------------- | ------------------ | ------------- |
| ssh auth          | external inventory | non-linux     |
| ssh agent         | facts              | sudo / become |
| ssh_config        | pip / env          | ssh passwords |
| ProxyJump         | roles              | local_action  |
| known_hosts       | rpm, yum, pacman   | with_xxx      |
| apt               | apt_repository     | delegate_to   |
| file/copy (tree)  | user               | set_fact      |
| dir               | mysql              | assert        |
| shell             | postgresql         | stat          |
| command           |                    | debug         |
| lineinfile        |                    |               |
| service           |                    |               |
| get_url           |                    |               |
| vars, vars_files  |                    |               |
| extra vars (`-e`) |                    |               |
| include           |                    |               |
| loop              |                    |               |
| templates         |                    |               |
| vault             |                    |               |
| validation        |                    |               |

# Changes from Ansible syntax

- `tree` module with state per line, and `template=false` / `state=absent` per line
- `apt` module with state per line
- `unless` for any task: a shell guard, skipped if it exits 0
- `creates` / `removes`: path guards, checked without a shell
- `changed_when`: decides whether a command counts as a change
- `include` instead of `import_playbook`
- no `when`, no `register`, no facts. If you need a condition, it is a guard

# Guards and idempotency

Whip has no `when`. A task is skipped by one of three guards, cheapest first:

```yaml
- name: unpack the release
  shell: tar xzf /tmp/app.tgz -C /srv/app
  creates: /srv/app/index.php      # skip if the path exists

- name: drop the old socket
  shell: rm /run/app.sock
  removes: /run/app.sock           # skip if the path is already gone

- name: add the repo key
  shell: curl -fsS https://example/key | gpg --dearmor -o /etc/apt/keyrings/x.gpg
  unless: test -s /etc/apt/keyrings/x.gpg
```

`shell` and `command` otherwise report *changed* every run, which makes the run
summary meaningless. Say what a change means:

```yaml
- shell: systemctl daemon-reload
  changed_when: false

- shell: /usr/local/bin/sync-catalog
  changed_when: 'echo "$WHIP_OUTPUT" | grep -q "^updated "'
```

# Variables

Four places, in increasing precedence: `vars_files`, play `vars`, task `vars`,
and `-e` on the command line. `hosts` is templated too, so one playbook runs
against any target:

```yaml
- hosts: ["root@{{ guest_ip }}"]   # quoted: [root@{{ x }}] is not valid YAML
  vars_files: [vars/common.yml]
  vars:
    guest_ip: 10.0.0.5
  tasks:
    - shell: hostname
```

```sh
whip site.yml -e guest_ip=10.0.0.9 -e docroot=/srv/other
```

# Playbook validation

A playbook that whip cannot make sense of is an error, not a warning. Whip
refuses to run when it finds:

- a field it does not recognise (`unles:`, `notifiy:`)
- an argument the runner does not read (`service: {name: x, stat: started}`)
- a `notify` naming a handler that does not exist
- an Ansible keyword that would change behaviour if it were honoured, such as
  `become` or `when`

Ansible keywords that are simply irrelevant here — `gather_facts`,
`remote_user`, `connection` — are accepted and ignored, so an Ansible playbook
still loads.

# SSH

Targets are `user@host`, `user@host:port`, or a `~/.ssh/config` alias. Whip
reads that config for `HostName`, `User`, `Port`, `IdentityFile`,
`IdentitiesOnly` and `ProxyJump`, so a fleet behind a bastion needs no tunnel:

```
Host web-*
    User root
    ProxyJump bastion.example
```

Point whip at a different file with `WHIP_SSH_CONFIG=/path/to/config`, which is
how a repository ships the stanzas its own playbooks need.

Keys are tried in this order: the agent, `IdentityFile` from the config, then
`id_ed25519`, `id_ecdsa`, `id_rsa`. Host keys are verified against
`known_hosts` with `accept-new` semantics; `--insecure` turns that off.

Full reference: [doc/usage.md](doc/usage.md).

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

#### Why does `become: true` fail instead of being ignored?

Because ignoring it is the dangerous option. A play written to escalate and
run without escalation does the wrong work as the wrong user, and then reports
success. Whip has no privilege escalation: connect as the user you need.

