# Chief Whip Devops Automation

Chief Whip is a _fast_ and _simple_ Ansible replacement optimized for projects with 1 to 20 servers. 

# Todo

- [x] deputy loads job from stdin
- [ ] build.sh embeds multi-arch linux deputy builds into chief
- [ ] chief loads inventory from file 
- [ ] chief connects to target
- [ ] chief tests presence of deputy over target
- [ ] chief runs deputy over target
- [ ] chief uploads deputy to target
- [ ] chief loads playbook into job
- [ ] chief tests playbook for syntax errors
- [ ] deputy runs "command" task
- [ ] chief deputy supports rsync (via this? https://github.com/gokrazy/rsync/)

# Demo

# Objectives

Fast!

1. Eliminate unnecessary SSH round trips: Ansibles biggest delay is caused by tasks that are sent one by one. Chief Whip bundles tasks into a single job. 
2. Replacing Python with Golang should give another speed boost while generating jobs.

# Simple

1. Support for core tasks that cover 98% of use cases 
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
    - apt
5. No more:
    - `gather_facts` option but instead lazy loading
    - `become`, only use the user as specified in the inventory 

# But why?

I really loved Ansible. Compared to the popular configuration management systems at the time (Puppet, Chef, CFEngine), it was a breeze of fresh air. Simple configuration files, easy to learn, effective documentation, simple push architecture. My team used it to manage some 2k+ servers without a fuss. 

Until version 2 or so. After the RedHat acquisition, Ansible has quickly grown into commercial bloatware. It's funny how RedHat got rid of the old objectives page (Simple, Fast) and replaced it with a corporate maze of marketing fluff. The task parameter documentation is hidden deep in the corporate site. Core modules have grown to support 20 extra options to support esoteric use cases. And above all, its once legendary speed is gone. Ansible feels sluggish today.

Fret no more, let's relive the original Ansible experience!