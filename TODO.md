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
- [ ] support for template substitution
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
- [ ] support vault
- [ ] add taskrunner syntax validation so we can lint the tasks before actual run
- [ ] struct based cli arg parsing? such as go-arg or go-flags or kong

# Ideal playbook

- hosts: ubuntu@192.168.64.16
  tasks:
  - files: base/host1 - /etc/nginx:
    handler: nginx
    owner: root -

# Todo for MVP / internal use

x bug: \_args get assigned to every task

- secrets (age?)
  x templates
- local command (go build)
- apt
  x handlers, notify
  x files: owner, state, notify
- lineinfile
  x files: actual checksum comparison
  x systemd (service?)

nice:

- tree: move assets from run param to arg param
- tree sync: use tar or std serialization
  x use gob instead of json for cmd streaming
- validate handler names
- alert on duplicate handlers
- exit 1 if any tasks errorred?
- replace Afero with tar for files serialization, so we can infer filemode from the src files
