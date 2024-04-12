# Chief Whip Devops Automation

![order, order!](doc/order-order.webp)

Whip your servers into line. Chief Whip is a _fast_ and _simple_ Ansible replacement optimized for projects with 1 to 50 servers.

# Demo

> [!NOTE] Keep this in mind.

# Objectives

Fast!

1. Eliminate unnecessary SSH round trips: Ansibles biggest delay is caused by tasks that are sent one by one. Chief Whip bundles tasks into a single job.
2. Replacing Python with Golang should give another speed boost while generating jobs.

Simple!

1. Support for core tasks that cover 98% of use cases (copy files, restart some processes)
2. Linux servers only
3. SSH support only
4. StrictYaml only
5. Tasks supported:
   - template
   - command
   - shell
   - copy
   - file
   - systemd
   - package
   - pip
6. No more:
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

- [I'm done with Red Hat](https://www.jeffgeerling.com/blog/2023/im-done-red-hat-enterprise-linux)
- [Is Ansible turing complete?](https://stackoverflow.com/questions/40127586/is-ansible-turing-complete)
- [gossh: declarative config management using Go](https://github.com/krilor/gossh)
- [Ansible's YAML file is essentially code](https://news.ycombinator.com/item?id=16238005)
- [Top Ansible tasks](https://mike42.me/blog/2019-01-the-top-100-ansible-modules)
- [Configuration complexity clock](http://mikehadlow.blogspot.com/2012/05/configuration-complexity-clock.html?m=1)

# FAQ

- Why is there an embedded build? To support different architectures between host and client
- Is Chief Whip designed to be an Ansible replacement (backwards compatible) or to be a better solution to the same problem?
