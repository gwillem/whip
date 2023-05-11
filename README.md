# Chief Whip Devops Automation

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
- [ ] rename Host to Target in code
- [x] chief runs jobs in parallel
- [ ] limit parallel jobs to x, cli argument
- [ ] add license
- [ ] ensure basic go doc
- [ ] support for inventory files
- [ ] support for variables
- [ ] support for template substitution
- [ ] support for "with_items"
- [ ] implement basic runners 
- [ ] bump ux with https://charm.sh/libs/
- [ ] publish on github
- [ ] add taskrunner syntax validation so we can lint the tasks before actual run
- [ ] chief tests playbook for syntax errors
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
    - support for plain password authentication (key/agent only)
    - `gather_facts` option but instead lazy loading
    - `become`, no sudo trickery

# But why?

I really loved Ansible. Compared to the popular configuration management systems at the time (Puppet, Chef, CFEngine), it was a breeze of fresh air. Simple configuration files, easy to learn, effective documentation, simple push architecture. My team used it to manage some 2k+ servers without a fuss. 

Until version 2 or so. After the RedHat acquisition, Ansible has quickly grown into commercial bloatware. It's funny how RedHat got rid of the old objectives page (Simple, Fast) and replaced it with a corporate bog of marketing fluff. The task parameter documentation is hidden behind compulsory white paper downloads. Core modules have grown to support 20 extra options to support esoteric use cases. And above all, its once legendary speed is gone. Ansible feels sluggish today.

Fret no more, let's relive the original Ansible experience!