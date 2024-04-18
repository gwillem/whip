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
  - [ ] shell
  - [ ] command
  - [ ] template
  - [ ] file
  - [ ] apt
  - [ ] systemd
  - [ ] rsync support (via this? https://github.com/gokrazy/rsync/)
- [ ] publish on github
- [x] support vault
- [ ] add taskrunner syntax validation so we can lint the tasks before actual run
- [ ] struct based cli arg parsing? such as go-arg or go-flags or kong

# Ideal playbook

- hosts: ubuntu@192.168.64.16
  tasks:
  - files: base/host1 - /etc/nginx:
    handler: nginx
    owner: root -

# Todo for MVP / internal use

- [x] bug: \_args get assigned to every task
- [x] bug: tree, prefixmap props don't trickle down in map (eg handler for /etc)
- [x] bug: task (pre-) runner should be able to modify their vars, args
- [x] secrets (age?)
- [x] templates
- local command (go build)
- [x] task pre-runners? could load files, run local commands etc
- [ ] apt
- [x] handlers, notify
- [x] files: owner, state, notify
- [ ] lineinfile
- [x] files: actual checksum comparison
- [x] systemd (service?)

nice:

- support for ansible vault https://github.com/sosedoff/ansible-vault-go/blob/master/vault.go
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
