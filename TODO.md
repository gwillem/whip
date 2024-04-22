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
- [x] support for variables
  - can be defined in playbook, task (via "loop") or externally?
- [x] add air for dev rebuild
- [x] actually sends files
- [x] replace json ipc with gob streaming
- [x] support for template substitution
- [ ] support for inventory files
- [x] chief also reads stderr from deputy to catch panics
- [ ] limit parallel jobs to x, cli argument
- [ ] record gif demo for in readme https://github.com/charmbracelet/vhs
- [x] support handlers
- [ ] implement basic runners https://mike42.me/blog/2019-01-the-top-100-ansible-modules
  - [ ] copy
  - [ ] template
  - [x] authorized_key
  - [x] shell
  - [x] command
  - [ ] template
  - [ ] file
  - [x] apt
  - [x] systemd
  - [ ] rsync support (via this? https://github.com/gokrazy/rsync/)
- [ ] publish on github
- [x] support vault
- [ ] add taskrunner syntax validation so we can lint the tasks before actual run
- [ ] struct based cli arg parsing? such as go-arg or go-flags or kong
- [ ] module auto doc generation

# Todo for MVP / internal use

- [ ] tests for vault
- [x] tui progress shows DONE when there is an error
- [ ] rename runner to module
- [ ] dont pass whole task to runner
- [ ] add: creates as backwards compat
- [ ] bug: tree does not work?
- [ ] better warning for unvalidated runner:

```
    - name: install root bashrc
      copy: src=files/root/bashrc dest=/root/.bashrc
gives
0.000 XXX Runner not found, should have been validated

```

- [ ] args vs \_args

```
    # - name: inital request of wildcard certificate
    #   command: "certbot certonly -n --agree-tos --email=info@sansec.io --dns-cloudflare --dns-cloudflare-credentials /etc/certbot/sans.ec.creds -d '*.sans.ec'"
    #   args:
    #     creates: /etc/letsencrypt/live/sans.ec/cert.pem
    #   tags: certbot
```

- [ ] get_url
- [ ] apt: state latest?
- [x] apt: pkg should be "name" ?
- [ ] tags
- [x] bug: \_args get assigned to every task
- [x] bug: tree, prefixmap props don't trickle down in map (eg handler for /etc)
- [x] bug: task (pre-) runner should be able to modify their vars, args
- [x] secrets (age?)
- [x] templates
- [x] local command (go build) -- hmmm should get rid of this
- [x] task pre-runners? could load files, run local commands etc
- [x] apt
- [x] creates/validates options ==> unless
- [x] handlers, notify
- [x] files: owner, state, notify
- [x] lineinfile
- [x] files: actual checksum comparison
- [x] systemd (service?)

nice:

x support for ansible vault https://github.com/sosedoff/ansible-vault-go/blob/master/vault.go

- fix tests
- tree: move assets from run param to arg param
- tree sync: use tar or std serialization
  x use gob instead of json for cmd streaming
- validate handler names
- alert on duplicate handlers
  x exit 1 if any tasks errorred?
- replace Afero with tar for files serialization, so we can infer filemode from the src files
- set up docs https://squidfunk.github.io/mkdocs-material/setup/adding-a-comment-system/

code smell

- composability: embed "install sansec ssh keys" ?
- embedded files per task
- flatten task list per target, kill play, just send list of tasks to deputy
- simplify tr, tr should not be responsible for counting fi
- don't send vars to runner, should be interpolated by deputy
  - how does a template with host facts get processed?
- need to validate key=val params for the tree module (and others?)
