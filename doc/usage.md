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

Hosts are direct SSH targets in `user@host` or `user@host:port` form. If `user` is omitted, local `$USER` is used. There is no external inventory file.

Authentication uses:

1. SSH agent via `$SSH_AUTH_SOCK`, and/or
2. `$HOME/.ssh/id_rsa`.

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
      shell: systemctl is-active nginx || systemctl start nginx
      unless: systemctl is-active nginx

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

Recognized play fields: `name`, `hosts`, `vars`, `prerun`, `tasks`, `handlers`.

Recognized task fields: `name`, runner key (`apt`, `shell`, etc.), `vars`, `loop`, `notify`, `unless`, `tags`.

Unknown fields are ignored with a warning; do not rely on Ansible-only fields such as `remote_user`, `become`, `gather_facts`, roles, or includes.

## Task syntax

A task selects exactly one runner by using the runner name as a key:

```yaml
- shell: echo hello
- command: hostname
- service:
    name: nginx
    state: restarted
```

String runner values are parsed into the default argument `_args`; simple `key=value` tokens become named args. Map values are passed as args directly. Prefer map syntax for anything non-trivial.

`unless` is a top-level task guard. It is executed on the target with `/bin/sh -c`; if it exits 0, the task is skipped.

`notify` may be a YAML list or comma-separated string. Handlers run once after the play if notified by a changed task.

`loop` expands a task once per item and exposes `{{ item }}`:

```yaml
- shell: echo {{ item }}
  loop: [one, two]
```

## Variables and templates

Variables are maps. Play vars and task vars are available to task argument templates and text files copied by `tree`.

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

`tree` also templates text files from its source directory before writing them to the target. Binary files are copied as-is.

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
```

Notes:

- `dst` may be absolute. Relative `dst` is under remote `$HOME`.
- Source file executable bits are preserved; broad file permissions are reduced by `umask`.
- Prefix metadata keys must start with `/` and apply to matching paths inside the source tree.
- Prefix metadata supports `owner`, `group`, `umask`, `notify`.
- Changed files can notify handlers through prefix metadata.

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
