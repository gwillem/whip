# Chief Whip Devops Automation

![order, order!](doc/order-order.webp)

Whip your servers into line. Chief Whip is a _fast_ and _simple_ Ansible replacement optimized for projects with 1 to 20 servers. 

# Todo

- [x] deputy can execute simple "command" task
- [x] chief connects to target
- [x] chief loads playbook into job
- [x] deputy loads job from stdin
- [x] chief and deputy can exchange job over ssh
- [x] build.sh embeds multi-arch linux deputy builds into chief
- [x] chief tests presence of deputy over target
- [x] chief uploads deputy to target
- [x] chief supports embedded targets in playbooks
- [x] support for with_items / loop
- [x] chief runs jobs in parallel
- [x] properly fix quoting in yaml parsing, maybe use this? https://pkg.go.dev/github.com/mitchellh/mapstructure#example-Decode
- [x] add license
- [x] display task results with "whip -v"
- [x] Deputy handles multi plays
- [ ] support for variables
    - can be defined in playbook, task (via "loop") or externally?
- [ ] support for template substitution
- [ ] rename Host to Target in code
- [ ] support for inventory files
- [ ] chief also reads stderr from deputy to catch panics
- [ ] limit parallel jobs to x, cli argument
- [ ] record gif demo for in readme  https://github.com/charmbracelet/vhs
- [ ] ensure basic go doc
- [ ] support handlers
- [ ] implement basic runners 
    - [ ] copy
    - [ ] template
    - [x] authorized_key 
    - [ ] shell
    - [ ] command
    - [ ] template
    - [ ] file
    - [ ] apt
    - [ ] systemd
    - [ ] rsync support (via this? https://github.com/gokrazy/rsync/)
- [ ] publish on github
- [ ] support vault
- [ ] add taskrunner syntax validation so we can lint the tasks before actual run
- [ ] struct based cli arg parsing? such as go-arg or go-flags or kong

# Demo

# Objectives

Fast!

1. Eliminate unnecessary SSH round trips: Ansibles biggest delay is caused by tasks that are sent one by one. Chief Whip bundles tasks into a single job. 
2. Replacing Python with Golang should give another speed boost while generating jobs.

# Simple

1. Support for core tasks that cover 98% of use cases (copy files, restart some processes)
2. Linux servers only
3. SSH support only
4. StrictYaml only
1. Tasks supported:
    - template
    - command
    - shell
    - copy
    - file
    - systemd
    - package
    - pip
5. No more:
    - support for plain password authentication (key/agent only)
    - `gather_facts` option but instead lazy loading
    - `become`, no sudo trickery
    - `with_<lookup>`
    - variables that can be overridden in 10 places

# But why?

I really loved Ansible. Compared to the popular configuration management systems at the time (Puppet, Chef, CFEngine), it was a breeze of fresh air. Simple configuration files, easy to learn, effective documentation, simple push architecture. My team used it to manage some 2k+ servers without a fuss. 

Until version 2 or so. After the RedHat acquisition, Ansible has quickly grown into commercial bloatware. It's funny how RedHat got rid of the old objectives page (Simple, Fast) and replaced it with a corporate bog of marketing fluff. The task parameter documentation is hidden behind compulsory white paper downloads. Core modules have grown to support 20 extra options to support esoteric use cases. And above all, its once legendary speed is gone. Ansible feels sluggish today.

Ansible has morphed from a declarative model to an imperative model, by supporting loops and control flow. 

> Any sufficiently complicated configuration language contains an ad hoc, informally-specified, bug-ridden, slow implementation of a Turing complete programming language. (jasim @ HN)

Fret no more, let's relive the original Ansible experience!

# Other reading

- [Is Ansible turing complete?](https://stackoverflow.com/questions/40127586/is-ansible-turing-complete)
- [gossh: declarative config management using Go](https://github.com/krilor/gossh)
- [Ansible's YAML file is essentially code](https://news.ycombinator.com/item?id=16238005)