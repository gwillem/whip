# Usage

This is the compact end-user contract for writing and running Whip playbooks, intended to be easy for LLM clients to consume.

## What Whip does

Whip runs YAML playbooks over SSH. The local `whip` binary loads the playbook, bundles tasks/assets, uploads or reuses a small remote `deputy` binary, then streams one job per target host. Targets run tasks locally and stream results back.

Supported clients: macOS/Linux. Supported targets: Linux over SSH.

## Project layout

Recommended layout:

```text
.whip/
  playbook.yml        # default playbook path
  secret.sh           # optional executable that prints WHIP_KEY
files/                # local file trees used by tree tasks
```

Path rules:

- Default playbook: `.whip/playbook.yml` in the current directory or any parent.
- You may pass another playbook path: `whip path/to/playbook.yml`.
- Whip changes working directory to the playbook's parent before loading assets.
- `tree.src` paths are local paths relative to the playbook directory.
- Remote deputy path: `$HOME/.cache/whip/deputy` on each target.
- Remote deputy stderr: `$HOME/.cache/whip/whip.err` on each target.

## Invocation

```sh
whip                         # find .whip/playbook.yml upward and run it
whip .whip/playbook.yml      # run explicit playbook
whip -v                      # task-level logging
whip -vv                     # debug logging
whip -e key=value            # set a variable, repeatable, beats the playbook
whip --insecure              # do not verify SSH host keys
whip --version
whip update                  # update to latest release
```

Secrets commands:

```sh
whip edit files/secret.conf          # edit encrypted/plain file, save encrypted
whip encrypt < plain > encrypted     # encrypt stdin to stdout
whip decrypt < encrypted > plain     # decrypt stdin to stdout
whip convert files/old-vault.yml     # convert Ansible Vault file to Whip/Age
```

## SSH and target requirements

Hosts are SSH targets in `user@host` or `user@host:port` form, or a `~/.ssh/config`
alias. If `user` is omitted, local `$USER` is used. There is no external inventory file.

`hosts` is templated, so a target may be built from variables:

```yaml
- hosts: ["root@{{ guest_ip }}"]   # quoted: [root@{{ x }}] is not valid YAML
```

Whip reads `~/.ssh/config` for `HostName`, `User`, `Port`, `IdentityFile`,
`IdentitiesOnly` and `ProxyJump`. Anything spelled out in the target string wins over
the config, as with `ssh(1)`. `ProxyJump` chains are followed recursively, and a jump
host is itself resolved through its own stanza, so a fleet behind a bastion needs no
tunnel and no forwarded ports:

```
Host guest-*
    User root
    ProxyJump bastion.example
```

Set `WHIP_SSH_CONFIG=/path/to/config` to read a different file. That is how a
repository ships the stanzas its own playbooks need, instead of asking every operator
to edit their dotfiles.

Authentication is tried in this order, skipping what is absent:

1. SSH agent via `$SSH_AUTH_SOCK`
2. every `IdentityFile` named for that host in the config
3. `~/.ssh/id_ed25519`, `~/.ssh/id_ecdsa`, `~/.ssh/id_rsa`

Host keys are checked against `~/.ssh/known_hosts`. An unknown host is accepted and
recorded on first contact (`StrictHostKeyChecking=accept-new`); a key that later
changes is refused, naming the host, both fingerprints and the `known_hosts` line.
`--insecure` skips the check entirely.

The remote job is invoked with `sudo $HOME/.cache/whip/deputy`; targets must allow that non-interactively, or you should connect as a user for which it works.

## Playbook format

A playbook is a YAML list of plays:

```yaml
- name: web servers
  hosts:
    - root@web1.example.com
    - root@web2.example.com:22
  vars:
    app_name: demo
  prerun:
    - echo runs locally before jobs are sent
  tasks:
    - name: install packages
      apt:
        name:
          - nginx
          - curl
        state: present

    - name: render/sync files
      tree:
        src: files/web
        dst: /
        /: owner=root group=root umask=022 notify=reload nginx
      notify: restart nginx

    - name: guarded shell command
      shell: tar xzf /tmp/app.tgz -C /srv/app
      creates: /srv/app/index.php

  handlers:
    - name: reload nginx
      service:
        name: nginx
        state: reloaded
    - name: restart nginx
      service:
        name: nginx
        state: restarted
```

Recognized play fields: `name`, `hosts`, `vars`, `vars_files`, `prerun`, `tasks`,
`handlers`, `assets`.

Recognized task fields: `name`, runner key (`apt`, `shell`, etc.), `vars`, `loop`,
`notify`, `unless`, `creates`, `removes`, `changed_when`, `tags`.

### Composing playbooks

`vars_files` merges YAML files into the play's variables, in order, with the play's
own `vars` winning. Paths are relative to the playbook, not to the working directory,
so a playbook means the same thing wherever it is run from.

`include` splices another playbook's plays in place. It must be the only key in its
list entry, and an included file may itself include, resolved relative to its own
directory:

```yaml
- include: common/bootstrap.yml
- name: web servers
  hosts: [root@web1.example.com]
  vars_files: [vars/common.yml, vars/production.yml]
  tasks:
    - shell: hostname
```

### Validation

A playbook whip cannot make sense of is an error, not a warning. It refuses to run on:

- a field it does not recognise (`unles:`, `notifiy:`)
- an argument the runner does not read (`service: {name: x, stat: started}`)
- a `notify` naming a handler that does not exist in that play
- an Ansible keyword that would change behaviour if honoured: `become`, `become_user`,
  `become_method`, `sudo`, `sudo_user`, `when`, `vars_prompt`

Ansible keywords that are merely irrelevant here are accepted and ignored, so an
Ansible playbook still loads: `gather_facts`, `remote_user`, `connection`,
`any_errors_fatal`, `serial`, `strategy`.

## Task syntax

A task selects exactly one runner by using the runner name as a key:

```yaml
- shell: echo hello
- command: hostname
- service:
    name: nginx
    state: restarted
```

A string value goes to the runner's default argument. For `shell` and `command` it is
taken verbatim, so a command survives intact:

```yaml
- shell: mysql -e "SET @a=1"      # arrives whole, '=' and all
```

Other runners keep the `key=value` dialect, where each `=`-bearing token becomes a
named argument and the rest is joined into `_args`. That is what `apt` package names
and `tree` prefix lines are written in. Map values are passed as args directly; prefer
the map form for anything non-trivial.

Three guards skip a task, evaluated in this order:

| Guard | Skips when | Notes |
| --- | --- | --- |
| `creates` | the path exists | a path, checked without a shell |
| `removes` | the path does not exist | a path, checked without a shell |
| `unless` | the command exits 0 | run on the target with `/bin/sh -c`, templated |

`changed_when` decides whether a task counts as a change: a shell expression evaluated
after the task, with the task's own output in `$WHIP_OUTPUT`, or a bool. Without it
`shell` and `command` always report changed.

```yaml
- shell: systemctl daemon-reload
  changed_when: false
```

`notify` may be a YAML list or comma-separated string. Handlers run once after the play if notified by a changed task.

`loop` expands a task once per item and exposes `{{ item }}`:

```yaml
- shell: echo {{ item }}
  loop: [one, two]
```

A loop item is itself rendered before substitution, so a variable inside an item
works:

```yaml
- lineinfile: {path: /etc/my.cnf, line: "{{ item }}"}
  loop: ["innodb_buffer_pool_size = {{ pool }}M"]
```

## Variables and templates

Variables are maps, available to task arguments, to `hosts`, to guards, and to text
files copied by `tree`. They come from four places, in increasing precedence:

| Source | Beaten by | Use for |
| --- | --- | --- |
| `vars_files` | everything below | a set shared by several playbooks |
| play `vars` | task vars, `-e` | the playbook's own defaults |
| task `vars` | `-e` | a value local to one task |
| `-e key=value` | nothing | per-run and per-target values |

`-e` is repeatable and always wins, so one playbook can serve several targets from a
script without being edited:

```sh
whip site.yml -e guest_ip=10.0.0.9 -e docroot=/srv/other
```

```yaml
- hosts: root@example.com
  vars:
    username: deploy
  tasks:
    - command: id {{ username }}
    - command: echo {{ item }}
      loop: [a, b]
```

Templates use Jinja-style `{{ name }}` via Gonja with strict undefined variables; referencing a missing variable fails the task.

`tree` also templates text files from its source directory before writing them to the
target. Binary files are copied as-is, and a prefix marked `template=false` is shipped
byte-for-byte, which is what a shell script full of `${VAR}` needs.

Templating is recursive: a variable inside a list or a map argument is rendered too.

## Runners

### `shell`

Runs a command through `/bin/sh -c`.

```yaml
- shell: echo hello > /tmp/hello
```

### `command`

Runs a command directly after shell-like splitting. Use for commands that do not require shell operators.

```yaml
- command: hostname
```

### `apt`

Ensures Debian/Ubuntu package state using `apt-get`.

```yaml
- apt:
    name:
      - nginx
      - curl state=absent
    state: present
```

States: `present` (default), `absent`, `purged`. Per-package `state=...` in `name` overrides the task state.

### `tree`

Synchronizes a local directory tree to a remote destination.

```yaml
- tree:
    src: files/etc
    dst: /
    /: owner=root group=root umask=022 notify=reload nginx
    /nginx: owner=root group=www-data umask=027
    /nginx/vendor: template=false
    /nginx/sites-enabled/old-shop: state=absent
```

Prefix metadata keys must start with `/` and apply to matching paths inside the
source tree. Longer, more specific prefixes override shorter ones; `notify`
accumulates.

| Key        | Values                | Default   | Meaning                                                          |
| ---------- | --------------------- | --------- | ---------------------------------------------------------------- |
| `owner`    | user name             | unchanged | Chown the subtree to this user                                    |
| `group`    | group name            | unchanged | Chgrp the subtree to this group                                   |
| `umask`    | octal, e.g. `027`     | `022`     | Reduce source permissions by this mask                            |
| `notify`   | handler names, comma  | none      | Handlers to run when something under the prefix changed           |
| `template` | `true`, `false`       | `true`    | Render text files as templates; `false` ships them byte-for-byte  |
| `state`    | `present`, `absent`   | `present` | `absent` removes the prefix from the target instead of shipping it |

Notes:

- `dst` may be absolute. Relative `dst` is under remote `$HOME`.
- A missing `dst` is created, with the root prefix's `umask` and owner.
- Source file executable bits are preserved; broad file permissions are reduced by `umask`.
- A prefix sets only what it names: without `owner` or `group` the subtree keeps its ownership.
- `state=absent` reports a change only when something was really removed, and
  refuses to be combined with another prefix inside it.

### `dir`

Ensures a directory exists with a given mode and owner, or is gone.

```yaml
- dir: /srv/honeypot/capture

- dir:
    path: /srv/honeypot/capture/bodies
    mode: "0750"
    owner: www-data
    group: root

- dir:
    path: /srv/honeypot/old
    state: absent
```

Notes:

- Parent directories are created as needed, as `install -d` does.
- `mode` is octal and must be quoted, otherwise YAML reads it as a decimal number.
- Mode and owner drift on an existing directory is corrected and reported as a change.

### `lineinfile`

Ensures one line exists in a file.

```yaml
- lineinfile:
    path: /etc/sysctl.conf
    line: net.ipv4.ip_forward=1
```

### `service`

Calls `systemctl`.

```yaml
- service:
    name: nginx
    state: restarted
```

States: `started`, `stopped`, `restarted`, `reloaded`.

### `get_url`

Downloads a URL to a destination path.

```yaml
- get_url:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
```

## Secrets handling

Whip secrets are encrypted files, primarily for files shipped through `tree`. Playbooks themselves are loaded as plain YAML.

Preferred encryption is Age with an identity in `WHIP_KEY`:

```sh
export WHIP_KEY='AGE-SECRET-KEY-...'
whip edit files/app.env
```

Alternatively create executable `.whip/secret.sh` that prints the Age identity:

```sh
#!/bin/sh
pass show whip/age-key
```

Ansible Vault decryption is supported for migration/compatibility through `ANSIBLE_VAULT_PASSWORD`:

```sh
export ANSIBLE_VAULT_PASSWORD='...'
whip convert files/secret.conf
```

When `tree` loads assets, encrypted files are decrypted locally, optionally templated if text, then sent inside the compressed job to the target. Keep `WHIP_KEY`/`.whip/secret.sh` out of version control.
